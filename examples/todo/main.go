package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-spring.dev/web"
)

// Todo represents a todo item
type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// In-memory store for todos
type TodoStore struct {
	mu     sync.RWMutex
	todos  map[int]Todo
	nextID int
}

var store = &TodoStore{
	todos:  make(map[int]Todo),
	nextID: 1,
}

// AddTodo adds a new todo to the store
func (s *TodoStore) AddTodo(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	todo := Todo{
		ID:        s.nextID,
		Title:     title,
		Completed: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.todos[s.nextID] = todo
	s.nextID++
	return todo
}

// GetTodo returns a todo by ID
func (s *TodoStore) GetTodo(id int) (Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	todo, exists := s.todos[id]
	return todo, exists
}

// GetAllTodos returns all todos
func (s *TodoStore) GetAllTodos() []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	todos := make([]Todo, 0, len(s.todos))
	for _, todo := range s.todos {
		todos = append(todos, todo)
	}
	return todos
}

// UpdateTodo updates a todo
func (s *TodoStore) UpdateTodo(id int, title string, completed bool) (Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	todo, exists := s.todos[id]
	if !exists {
		return Todo{}, false
	}

	todo.Title = title
	todo.Completed = completed
	todo.UpdatedAt = time.Now()
	s.todos[id] = todo
	return todo, true
}

// DeleteTodo removes a todo
func (s *TodoStore) DeleteTodo(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.todos[id]
	delete(s.todos, id)
	return exists
}

func main() {
	router := web.NewRouter()

	// Global middleware: request logging
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)
			fmt.Printf("[%s] %s %s - %v\n", time.Now().Format("2006-01-02 15:04:05"), r.Method, r.URL.Path, duration)
		})
	})

	// Global middleware: CORS
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// API v1 group
	router.Group("/api/v1", func(r web.Router) {
		// Todos resource
		r.Group("/todos", func(r web.Router) {
			// GET /api/v1/todos - List all todos
			r.Get("/", func(ctx context.Context) ([]Todo, error) {
				return store.GetAllTodos(), nil
			})

			// GET /api/v1/todos/{id} - Get a specific todo
			r.Get("/{id}", func(ctx context.Context, req struct {
				ID int `path:"id"`
			}) (interface{}, error) {
				todo, exists := store.GetTodo(req.ID)
				if !exists {
					return nil, web.Error(404, "Todo not found")
				}
				return todo, nil
			})

			// POST /api/v1/todos - Create a new todo
			r.Post("/", func(ctx context.Context, req struct {
				Title string `json:"title"`
			}) (Todo, error) {
				if req.Title == "" {
					return Todo{}, web.Error(400, "Title is required")
				}
				return store.AddTodo(req.Title), nil
			})

			// PUT /api/v1/todos/{id} - Update a todo
			r.Put("/{id}", func(ctx context.Context, req struct {
				ID        int    `path:"id"`
				Title     string `json:"title"`
				Completed bool   `json:"completed"`
			}) (interface{}, error) {
				todo, exists := store.UpdateTodo(req.ID, req.Title, req.Completed)
				if !exists {
					return nil, web.Error(404, "Todo not found")
				}
				return todo, nil
			})

			// DELETE /api/v1/todos/{id} - Delete a todo
			r.Delete("/{id}", func(ctx context.Context, req struct {
				ID int `path:"id"`
			}) (map[string]interface{}, error) {
				exists := store.DeleteTodo(req.ID)
				if !exists {
					return nil, web.Error(404, "Todo not found")
				}
				return map[string]interface{}{
					"message": "Todo deleted successfully",
					"id":      req.ID,
				}, nil
			})
		})

		// Health check endpoint
		r.Get("/health", func(ctx context.Context) map[string]interface{} {
			return map[string]interface{}{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
				"todos":     len(store.todos),
			}
		})
	})

	// Web interface
	router.Get("/", func(ctx context.Context) {
		web.FromContext(ctx).HTML(200, `<!DOCTYPE html>
<html>
<head>
    <title>Todo API Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .method { display: inline-block; padding: 5px 10px; border-radius: 3px; color: white; font-weight: bold; }
        .get { background: #4CAF50; }
        .post { background: #2196F3; }
        .put { background: #FF9800; }
        .delete { background: #F44336; }
        code { background: #eee; padding: 2px 4px; border-radius: 3px; }
        pre { background: #2d2d2d; color: #f8f8f2; padding: 15px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Todo API Example</h1>
        <p>This example demonstrates a full REST API for managing todos using go-spring/web.</p>

        <h2>Available Endpoints</h2>

        <div class="endpoint">
            <span class="method get">GET</span> <code>/api/v1/todos</code>
            <p>List all todos</p>
        </div>

        <div class="endpoint">
            <span class="method get">GET</span> <code>/api/v1/todos/{id}</code>
            <p>Get a specific todo by ID</p>
        </div>

        <div class="endpoint">
            <span class="method post">POST</span> <code>/api/v1/todos</code>
            <p>Create a new todo</p>
            <pre>{"title": "Buy groceries"}</pre>
        </div>

        <div class="endpoint">
            <span class="method put">PUT</span> <code>/api/v1/todos/{id}</code>
            <p>Update a todo</p>
            <pre>{"title": "Buy groceries", "completed": true}</pre>
        </div>

        <div class="endpoint">
            <span class="method delete">DELETE</span> <code>/api/v1/todos/{id}</code>
            <p>Delete a todo</p>
        </div>

        <div class="endpoint">
            <span class="method get">GET</span> <code>/api/v1/health</code>
            <p>Health check endpoint</p>
        </div>

        <h2>Try It Out</h2>
        <p>Use curl or your favorite API client to test the endpoints:</p>
        <pre>
# List all todos
curl http://localhost:8080/api/v1/todos

# Create a new todo
curl -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title": "Learn go-spring/web"}'

# Update a todo
curl -X PUT http://localhost:8080/api/v1/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"title": "Learn go-spring/web", "completed": true}'

# Delete a todo
curl -X DELETE http://localhost:8080/api/v1/todos/1
        </pre>

        <h2>Features Demonstrated</h2>
        <ul>
            <li>✅ Automatic request binding from JSON, path, query parameters</li>
            <li>✅ Route grouping with <code>/api/v1</code> and <code>/todos</code> groups</li>
            <li>✅ Global middleware (logging, CORS)</li>
            <li>✅ Multiple HTTP methods (GET, POST, PUT, DELETE)</li>
            <li>✅ Structured error handling with <code>web.Error()</code></li>
            <li>✅ Automatic JSON response rendering</li>
            <li>✅ In-memory data store with thread safety</li>
        </ul>
    </div>
</body>
</html>`)
	})

	// Add some sample data
	store.AddTodo("Learn go-spring/web")
	store.AddTodo("Build a REST API")
	store.AddTodo("Write documentation")

	fmt.Println("Todo API server started at http://localhost:8080")
	fmt.Println("Open http://localhost:8080 in your browser for API documentation")
	fmt.Println("API endpoints available at /api/v1/todos")
	http.ListenAndServe(":8080", router)
}
