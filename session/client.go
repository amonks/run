package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// TaskInfo represents a task's status as returned by the session API.
type TaskInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Log    bool   `json:"log"`
}

// StatusResponse is the response from GET /tasks.
type StatusResponse struct {
	Session string     `json:"session"`
	Tasks   []TaskInfo `json:"tasks"`
}

// Client connects to a running session's Unix domain socket.
type Client struct {
	http *http.Client
	sock string
}

// Connect creates a client connected to the session identified by name
// and dir. Returns an error if the socket does not exist or is not
// reachable.
func Connect(name string, dir string) (*Client, error) {
	sock := SocketPath(name, dir)
	return connectTo(sock)
}

func connectTo(sock string) (*Client, error) {
	c := &Client{
		sock: sock,
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sock)
				},
			},
			Timeout: 5 * time.Second,
		},
	}

	// Verify the socket is reachable.
	resp, err := c.http.Get("http://localhost/tasks")
	if err != nil {
		return nil, fmt.Errorf("session '%s' is not reachable: %w", sock, err)
	}
	resp.Body.Close()

	return c, nil
}

// Status returns the status of all tasks in the session.
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.http.Get("http://localhost/tasks")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return &sr, nil
}

// EnableLog enables file logging for a task. Returns the log file path.
func (c *Client) EnableLog(taskID string) (string, error) {
	resp, err := c.http.Post("http://localhost/log/"+taskID, "", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("task %q not found", taskID)
	}

	var body struct {
		Path string `json:"path"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	return body.Path, nil
}

// DisableLog disables file logging for a task.
func (c *Client) DisableLog(taskID string) error {
	req, err := http.NewRequest("DELETE", "http://localhost/log/"+taskID, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Restart restarts a task.
func (c *Client) Restart(taskID string) error {
	resp, err := c.http.Post("http://localhost/restart/"+taskID, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("task %q not found", taskID)
	}
	return nil
}
