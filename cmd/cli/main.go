package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	forumURL       = os.Getenv("FORUM_URL")
	forumAgentName = os.Getenv("FORUM_AGENT_NAME")
	httpClient     = &http.Client{Timeout: 10 * time.Second}
)

type apiError struct {
	Error string `json:"error"`
}

type memberResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type topicResponse struct {
	ID        int64           `json:"id"`
	Title     string          `json:"title"`
	Content   string          `json:"content"`
	Status    string          `json:"status"`
	Creator   *memberResponse `json:"creator,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Replies   []replyResponse `json:"replies,omitempty"`
}

type replyResponse struct {
	ID      int64           `json:"id"`
	Content string          `json:"content"`
	Author  *memberResponse `json:"author,omitempty"`
}

type notificationResponse struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	TargetID int64  `json:"target_id"`
	Read     bool   `json:"read"`
}

func main() {
	if forumURL == "" {
		forumURL = "http://localhost:8080"
	}

	rootCmd := &cobra.Command{
		Use:   "forumctl",
		Short: "Agent Forum CLI Client",
	}

	rootCmd.PersistentFlags().StringVar(&forumURL, "url", forumURL, "Forum API base URL")
	rootCmd.PersistentFlags().StringVar(&forumAgentName, "agent", forumAgentName, "Agent name for API calls")

	rootCmd.AddCommand(memberCmd())
	rootCmd.AddCommand(topicCmd())
	rootCmd.AddCommand(replyCmd())
	rootCmd.AddCommand(checkCmd())
	rootCmd.AddCommand(notifyCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func memberCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "member", Short: "Member operations"}
	registerCmd := &cobra.Command{
		Use:   "register <name> <workspace>",
		Short: "Register a new member",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]string{"name": args[0], "workspace": args[1]}
			var result memberResponse
			if err := doJSON(http.MethodPost, "/api/members/register", payload, false, &result); err != nil {
				return err
			}
			fmt.Printf("Member registered: %s (ID: %d)\n", result.Name, result.ID)
			return nil
		},
	}
	cmd.AddCommand(registerCmd)
	return cmd
}

func topicCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "topic", Short: "Topic operations"}

	createCmd := &cobra.Command{
		Use:   "create <title> --content <content> --mention @xxx",
		Short: "Create a new topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _ := cmd.Flags().GetString("content")
			mentions, _ := cmd.Flags().GetStringSlice("mention")
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("--content is required")
			}

			payload := map[string]any{
				"title":    args[0],
				"content":  content,
				"mentions": normalizeMentions(mentions),
			}

			var result topicResponse
			if err := doJSON(http.MethodPost, "/api/topics", payload, true, &result); err != nil {
				return err
			}
			fmt.Printf("Topic created: #%d - %s\n", result.ID, result.Title)
			return nil
		},
	}
	createCmd.Flags().StringP("content", "c", "", "Topic content")
	createCmd.Flags().StringSliceP("mention", "m", []string{}, "Mention members (e.g., --mention @alice --mention @bob)")

	listCmd := &cobra.Command{
		Use:   "list [--page N] [--limit N] [--status open|closed]",
		Short: "List topics",
		RunE: func(cmd *cobra.Command, args []string) error {
			page, _ := cmd.Flags().GetInt("page")
			limit, _ := cmd.Flags().GetInt("limit")
			status, _ := cmd.Flags().GetString("status")

			values := url.Values{}
			values.Set("page", strconv.Itoa(page))
			values.Set("limit", strconv.Itoa(limit))
			if status != "" {
				values.Set("status", status)
			}

			var result []topicResponse
			if err := doJSON(http.MethodGet, "/api/topics?"+values.Encode(), nil, false, &result); err != nil {
				return err
			}

			fmt.Println("Topics:")
			fmt.Println("ID\tStatus\tTitle\tCreator\tCreated At")
			fmt.Println("----\t------\t-----\t-------\t----------")
			for _, t := range result {
				creator := ""
				if t.Creator != nil {
					creator = t.Creator.Name
				}
				createdAt := t.CreatedAt
				if len(createdAt) > 10 {
					createdAt = createdAt[:10]
				}
				fmt.Printf("%d\t%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Title, creator, createdAt)
			}
			fmt.Printf("\nTotal: %d topics\n", len(result))
			return nil
		},
	}
	listCmd.Flags().Int("page", 1, "Page number")
	listCmd.Flags().Int("limit", 20, "Limit per page")
	listCmd.Flags().String("status", "", "Filter by status (open/closed)")

	viewCmd := &cobra.Command{
		Use:   "view <id>",
		Short: "View topic details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result topicResponse
			if err := doJSON(http.MethodGet, "/api/topics/"+args[0], nil, false, &result); err != nil {
				return err
			}

			fmt.Printf("Topic #%d\n", result.ID)
			fmt.Printf("Title:   %s\n", result.Title)
			fmt.Printf("Status:  %s\n", result.Status)
			fmt.Printf("Content: %s\n", result.Content)
			if result.Creator != nil {
				fmt.Printf("Creator: %s\n", result.Creator.Name)
			}
			fmt.Printf("Created: %s\n", result.CreatedAt)
			fmt.Printf("Updated: %s\n", result.UpdatedAt)

			if len(result.Replies) > 0 {
				fmt.Println("\nReplies:")
				for _, reply := range result.Replies {
					author := "unknown"
					if reply.Author != nil {
						author = reply.Author.Name
					}
					fmt.Printf("  - %s: %s\n", author, reply.Content)
				}
			}
			return nil
		},
	}

	closeCmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Close a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return fmt.Errorf("invalid topic id: %w", err)
			}
			if err := doJSON(http.MethodPut, "/api/topics/"+args[0]+"/close", map[string]any{}, true, nil); err != nil {
				return err
			}
			fmt.Printf("Topic #%s closed\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, viewCmd, closeCmd)
	return cmd
}

func replyCmd() *cobra.Command {
	var replyTo int64
	cmd := &cobra.Command{
		Use:   "reply <topic_id> <content>",
		Short: "Reply to a topic",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return fmt.Errorf("invalid topic_id: must be a number")
			}

			payload := map[string]any{"content": args[1]}
			if replyTo > 0 {
				payload["reply_to_id"] = replyTo
			}
			var result []replyResponse
			if err := doJSON(http.MethodPost, "/api/topics/"+args[0]+"/replies", payload, true, &result); err != nil {
				return err
			}
			fmt.Printf("Reply added to topic #%s\n", args[0])
			return nil
		},
	}
	cmd.Flags().Int64Var(&replyTo, "reply-to", 0, "Reply to a specific reply ID")
	return cmd
}

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check mentions (for polling)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var topics []topicResponse
			if err := doJSON(http.MethodGet, "/api/agents/mentions", nil, true, &topics); err != nil {
				return err
			}
			if len(topics) == 0 {
				fmt.Println("No new mentions")
				return nil
			}

			fmt.Printf("You were mentioned in %d topic(s):\n", len(topics))
			for _, topic := range topics {
				creator := "unknown"
				if topic.Creator != nil {
					creator = topic.Creator.Name
				}
				fmt.Printf("  - #%d: %s (by %s)\n", topic.ID, topic.Title, creator)
			}
			return nil
		},
	}
}

func notifyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "notify", Short: "Notification operations"}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "View notification list",
		RunE: func(cmd *cobra.Command, args []string) error {
			var notifications []notificationResponse
			if err := doJSON(http.MethodGet, "/api/notifications", nil, true, &notifications); err != nil {
				return err
			}
			if len(notifications) == 0 {
				fmt.Println("No notifications")
				return nil
			}

			fmt.Println("Notifications:")
			fmt.Println("ID\tType\tTarget\tRead")
			fmt.Println("--\t----\t------\t----")
			for _, n := range notifications {
				readStatus := "unread"
				if n.Read {
					readStatus = "read"
				}
				fmt.Printf("%d\t%s\t%d\t%s\n", n.ID, n.Type, n.TargetID, readStatus)
			}
			return nil
		},
	}

	readCmd := &cobra.Command{
		Use:   "read <id> [id...]",
		Short: "Mark notifications as read",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]int64, 0, len(args))
			for _, arg := range args {
				id, err := strconv.ParseInt(arg, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid notification id %q: %w", arg, err)
				}
				ids = append(ids, id)
			}
			payload := map[string]any{"ids": ids}
			if err := doJSON(http.MethodPut, "/api/notifications/read", payload, true, nil); err != nil {
				return err
			}
			fmt.Printf("Marked %d notification(s) as read\n", len(ids))
			return nil
		},
	}

	cmd.AddCommand(listCmd, readCmd)
	cmd.RunE = listCmd.RunE
	return cmd
}

func doJSON(method, path string, payload any, requireAgent bool, out any) error {
	if requireAgent && strings.TrimSpace(forumAgentName) == "" {
		return fmt.Errorf("FORUM_AGENT_NAME is not set")
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, strings.TrimRight(forumURL, "/")+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(forumAgentName) != "" {
		req.Header.Set("X-Agent-Name", forumAgentName)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return fmt.Errorf("api error (%d): %s", resp.StatusCode, apiErr.Error)
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 {
			return fmt.Errorf("api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		return fmt.Errorf("api error (%d)", resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func normalizeMentions(mentions []string) []string {
	result := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		mention = strings.TrimSpace(strings.TrimPrefix(mention, "@"))
		if mention == "" {
			continue
		}
		result = append(result, mention)
	}
	return result
}
