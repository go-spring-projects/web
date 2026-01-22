package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go-spring.dev/web"
)

func main() {
	router := web.NewRouter()

	// SSE endpoint that sends server time every second
	router.Get("/sse/time", func(ctx context.Context) error {
		// Get the web context to access ResponseWriter
		wc := web.FromContext(ctx)

		// Create SSE sender
		sse, err := web.NewSSE(wc.Writer)
		if err != nil {
			return web.Error(500, "Failed to create SSE connection: "+err.Error())
		}

		// Send initial event
		sse.SendJSON("connected", map[string]interface{}{
			"message": "SSE connection established",
			"time":    time.Now().Format(time.RFC3339),
		})

		// Send retry interval (3 seconds)
		sse.SendRetry(3000)

		// Send time updates every second
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Client disconnected
				sse.SendJSON("disconnected", map[string]interface{}{
					"message": "Client disconnected",
					"time":    time.Now().Format(time.RFC3339),
				})
				sse.Close()
				return nil
			case t := <-ticker.C:
				// Send time update
				sse.SendJSON("time", map[string]interface{}{
					"timestamp": t.Unix(),
					"formatted": t.Format(time.RFC3339),
				})

				// Send a comment every 5 seconds (client ignores but keeps connection alive)
				if t.Second()%5 == 0 {
					sse.SendComment(fmt.Sprintf("Heartbeat at %s", t.Format(time.RFC3339)))
				}
			}
		}
	})

	// HTML page to test SSE
	router.Get("/", func(ctx context.Context) {
		web.FromContext(ctx).HTML(200, `<!DOCTYPE html>
<html>
<head>
    <title>SSE Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        #events {
            border: 1px solid #ccc;
            padding: 20px;
            height: 400px;
            overflow-y: auto;
            margin-top: 20px;
            background: #f9f9f9;
        }
        .event {
            padding: 8px;
            margin: 4px 0;
            background: white;
            border-left: 4px solid #4CAF50;
        }
        .event.error { border-left-color: #f44336; }
        .event.info { border-left-color: #2196F3; }
    </style>
</head>
<body>
    <h1>Server-Sent Events Example</h1>
    <p>This example demonstrates real-time server updates using SSE.</p>

    <button onclick="connectSSE()">Connect to SSE</button>
    <button onclick="disconnectSSE()">Disconnect</button>

    <div id="events"></div>

    <script>
        let eventSource = null;

        function connectSSE() {
            if (eventSource) {
                addEvent('Already connected', 'info');
                return;
            }

            eventSource = new EventSource('/sse/time');
            addEvent('Connecting to SSE endpoint...', 'info');

            eventSource.onopen = function() {
                addEvent('Connection opened successfully', 'info');
            };

            eventSource.onmessage = function(event) {
                addEvent('Message: ' + event.data, 'info');
            };

            eventSource.addEventListener('connected', function(event) {
                const data = JSON.parse(event.data);
                addEvent('Connected: ' + data.message + ' at ' + data.time, 'info');
            });

            eventSource.addEventListener('time', function(event) {
                const data = JSON.parse(event.data);
                addEvent('Time update: ' + data.formatted + ' (timestamp: ' + data.timestamp + ')', 'info');
            });

            eventSource.addEventListener('disconnected', function(event) {
                const data = JSON.parse(event.data);
                addEvent('Disconnected: ' + data.message + ' at ' + data.time, 'info');
                eventSource.close();
                eventSource = null;
            });

            eventSource.onerror = function(error) {
                addEvent('Error occurred: ' + JSON.stringify(error), 'error');
                if (eventSource.readyState === EventSource.CLOSED) {
                    eventSource.close();
                    eventSource = null;
                }
            };
        }

        function disconnectSSE() {
            if (eventSource) {
                eventSource.close();
                addEvent('Manually disconnected', 'info');
                eventSource = null;
            } else {
                addEvent('Not connected', 'info');
            }
        }

        function addEvent(text, type = 'info') {
            const eventsDiv = document.getElementById('events');
            const eventDiv = document.createElement('div');
            eventDiv.className = 'event ' + type;
            eventDiv.textContent = new Date().toLocaleTimeString() + ': ' + text;
            eventsDiv.appendChild(eventDiv);
            eventsDiv.scrollTop = eventsDiv.scrollHeight;
        }

        // Auto-connect after 1 second
        //setTimeout(connectSSE, 1000);
    </script>
</body>
</html>`)
	})

	fmt.Println("SSE server started at http://localhost:8080")
	fmt.Println("Open http://localhost:8080 in your browser to see SSE in action")
	http.ListenAndServe(":8080", router)
}
