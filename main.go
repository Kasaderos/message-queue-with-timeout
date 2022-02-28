package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

var MaxQueueSize = 100
var MaxQueueReaders = 100
var QueueRPS = 10

var port = flag.String("port", "8080", "port of queue service")

type message string

type reader struct {
	ch  chan message
	ctx context.Context
}

type queue struct {
	messages chan message
	readers  chan reader

	// rate limiter 10 per/sec with 100 bursts
	limiter *rate.Limiter
}

func (q *queue) push(msg message) {
	q.limiter.Wait(context.Background())
	for {
		select {
		case r := <-q.readers:
			select {
			case <-r.ctx.Done():
			case r.ch <- msg:
				return
			}
		default:

			q.messages <- msg
			return
		}
	}
}

func (q *queue) pop(seconds int) message {
	q.limiter.Wait(context.Background())
	select {
	case v := <-q.messages:
		return v
	default:
		// no message wait seconds

		ctx, _ := context.WithTimeout(context.Background(), time.Second*time.Duration(seconds))
		r := reader{
			ch:  make(chan message),
			ctx: ctx,
		}
		q.readers <- r

		select {
		case <-r.ctx.Done():
			return ""
		case v := <-r.ch:
			return v
		}
	}
}

type queueService struct {
	mu     sync.RWMutex
	queues map[string]queue
}

func NewQueueService() http.Handler {
	return &queueService{
		queues: make(map[string]queue, 0),
	}
}

func (srv *queueService) get(name string) queue {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	q, ok := srv.queues[name]
	if !ok {
		q = queue{
			messages: make(chan message, MaxQueueSize),
			readers:  make(chan reader, MaxQueueReaders),
			limiter:  rate.NewLimiter(rate.Limit(QueueRPS), 100),
		}
		srv.queues[name] = q
		return q
	}
	return q
}

func getName(path string) (name string, empty bool) {
	if len(path) == 0 {
		return "", true
	}
	// "/queue" -> "queue"
	path = path[1:]
	return path, false
}

func (srv *queueService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		name, empty := getName(r.URL.Path)
		v := message(r.URL.Query().Get("v"))
		if empty || len(v) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		q := srv.get(name)
		q.push(v)

		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		name, empty := getName(r.URL.Path)
		timeoutString := r.URL.Query().Get("timeout")
		if empty {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		q := srv.get(name)

		seconds, err := strconv.Atoi(timeoutString)
		if len(timeoutString) > 0 && err != nil {
			log.Printf("invalid timeout %v, trying without timeouts", err)
		}

		msg := q.pop(seconds)
		if len(msg) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(msg))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Recovered %v", err)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	service := NewQueueService()

	mux := http.NewServeMux()
	service = middleware(service)
	mux.Handle("/", service)

	log.Printf("listening port %v", *port)
	log.Fatal(http.ListenAndServe(":"+*port, mux))
}
