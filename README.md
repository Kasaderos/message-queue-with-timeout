### Simple implementation message queue (only standard library)

#### TASK
Implement a queue broker as a web service.
The service should handle 2 methods:
1. PUT /queue?v=....

put the message in a queue named queue (the queue name can be anything),

example:
```
curl -XPUT http://127.0.0.1/color?v=red
curl -XPUT http://127.0.0.1/color?v=green
curl -XPUT http://127.0.0.1/name?v=alex
curl -XPUT http://127.0.0.1/name?v=anna
```

in response {empty body + status 200 (ok)}
in the absence of the v parameter - an empty body + status 400 (bad request)



2. GET /queue

pick up (according to the FIFO principle) a message from the queue with the name queue and return it in the body
http request, example (the result that should be when the puts are executed above):
```
curl http://127.0.0.1/color => red
curl http://127.0.0.1/color => green
curl http://127.0.0.1/color => {empty body + status 404 (not found)}
curl http://127.0.0.1/color => {empty body + status 404 (not found)}
curl http://127.0.0.1/name => alex
curl http://127.0.0.1/name => anna
curl http://127.0.0.1/name => {empty body + status 404 (not found)}
```
for GET requests, make it possible to set the timeout argument
```
curl http://127.0.0.1/color?timeout=N
```
if there is no ready message in the queue, the recipient must wait either until
message arrival or until the timeout expires (N - number of seconds). If
the message never appeared - return a 404 code.
recipients must receive messages in the same order as they received the request,
if 2 recipients are waiting for a message (using a timeout), then the first message should
get the one who first requested.
