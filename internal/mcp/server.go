// Package mcp exposes glplan as a Model Context Protocol server over stdio.
package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"glplan/internal/api"
	"glplan/internal/model"
)

// Run starts the stdio MCP server.
func Run(baseURL, token, version string) error {
	c := api.NewClient(baseURL, token)
	s := server.NewMCPServer(
		"glplan",
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	registerTools(s, c)
	registerPrompts(s)
	fmt.Fprintln(os.Stderr, "glplan MCP server ready (stdio)")
	return server.ServeStdio(s)
}

type handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func add(s *server.MCPServer, tool mcp.Tool, fn func(mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return fn(req)
	})
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultJSON(v)
}

func fail(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

func projectArg() mcp.ToolOption {
	return mcp.WithString("project_id", mcp.Description("Project id or key (optional; defaults to the first project)"))
}

func resolveProject(c *api.Client, req mcp.CallToolRequest) (string, error) {
	id := req.GetString("project_id", "")
	if id != "" {
		return id, nil
	}
	projects, err := c.ListProjects()
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found; create one with create_project")
	}
	return projects[0].ID, nil
}

func registerTools(s *server.MCPServer, c *api.Client) {
	add(s, mcp.NewTool("list_projects",
		mcp.WithDescription("List all glplan projects (workspaces with a kanban board)."),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projects, err := c.ListProjects()
		if err != nil {
			return fail(err)
		}
		return jsonResult(projects)
	})

	add(s, mcp.NewTool("create_project",
		mcp.WithDescription("Create a new project with a default kanban board."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("key", mcp.Description("Short uppercase key used in task refs, e.g. API")),
		mcp.WithString("description", mcp.Description("Optional description")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return fail(err)
		}
		p, err := c.CreateProject(name, req.GetString("key", ""), req.GetString("description", ""))
		if err != nil {
			return fail(err)
		}
		return jsonResult(p)
	})

	add(s, mcp.NewTool("get_board",
		mcp.WithDescription("Get a project board: columns, labels, members and task counts."),
		projectArg(),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		b, err := c.GetBoard(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(b)
	})

	add(s, mcp.NewTool("list_columns",
		mcp.WithDescription("List the kanban columns of a project."),
		projectArg(),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		cols, err := c.ListColumns(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(cols)
	})

	add(s, mcp.NewTool("create_column",
		mcp.WithDescription("Add a column to a project board."),
		projectArg(),
		mcp.WithString("name", mcp.Required(), mcp.Description("Column name")),
		mcp.WithString("color", mcp.Description("Hex color, e.g. #3b82f6")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		name, err := req.RequireString("name")
		if err != nil {
			return fail(err)
		}
		col, err := c.CreateColumn(id, name, req.GetString("color", ""), 0)
		if err != nil {
			return fail(err)
		}
		return jsonResult(col)
	})

	add(s, mcp.NewTool("list_tasks",
		mcp.WithDescription("List tasks with optional filters. Returns full task objects including labels and subtasks."),
		projectArg(),
		mcp.WithString("column_id", mcp.Description("Filter by column id")),
		mcp.WithString("assignee_id", mcp.Description("Filter by member id")),
		mcp.WithString("label_id", mcp.Description("Filter by label id")),
		mcp.WithString("priority", mcp.Description("none|low|medium|high|urgent"), mcp.Enum("none", "low", "medium", "high", "urgent")),
		mcp.WithString("q", mcp.Description("Text search in title, description and reference")),
		mcp.WithBoolean("include_done", mcp.Description("Include completed tasks (default false)")),
		mcp.WithNumber("limit", mcp.Description("Max results (default: all)")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		tasks, err := c.ListTasks(model.TaskFilter{
			ProjectID:   id,
			ColumnID:    req.GetString("column_id", ""),
			AssigneeID:  req.GetString("assignee_id", ""),
			LabelID:     req.GetString("label_id", ""),
			Priority:    model.Priority(req.GetString("priority", "")),
			Search:      req.GetString("q", ""),
			IncludeDone: req.GetBool("include_done", false),
			Limit:       req.GetInt("limit", 0),
		})
		if err != nil {
			return fail(err)
		}
		return jsonResult(tasks)
	})

	add(s, mcp.NewTool("get_task",
		mcp.WithDescription("Get one task by id or reference such as GL-12."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		t, err := c.GetTask(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(t)
	})

	add(s, mcp.NewTool("create_task",
		mcp.WithDescription("Create a task on the board. Use column_id from get_board/list_columns."),
		projectArg(),
		mcp.WithString("title", mcp.Required(), mcp.Description("Task title")),
		mcp.WithString("description", mcp.Description("Markdown description")),
		mcp.WithString("column_id", mcp.Description("Target column (defaults to first non-done column)")),
		mcp.WithString("priority", mcp.Description("none|low|medium|high|urgent"), mcp.Enum("none", "low", "medium", "high", "urgent")),
		mcp.WithNumber("estimate", mcp.Description("Estimate in points")),
		mcp.WithArray("labels", mcp.Description("Label ids"), mcp.WithStringItems()),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		title, err := req.RequireString("title")
		if err != nil {
			return fail(err)
		}
		priority := req.GetString("priority", "none")
		t, err := c.CreateTask(api.CreateTaskInput{
			ProjectID:   project,
			ColumnID:    req.GetString("column_id", ""),
			Title:       title,
			Description: req.GetString("description", ""),
			Priority:    model.Priority(priority),
			Estimate:    req.GetInt("estimate", 0),
			LabelIDs:    req.GetStringSlice("labels", nil),
		})
		if err != nil {
			return fail(err)
		}
		return jsonResult(t)
	})

	add(s, mcp.NewTool("update_task",
		mcp.WithDescription("Update one or more task fields. Only provided fields change."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("priority", mcp.Description("none|low|medium|high|urgent"), mcp.Enum("none", "low", "medium", "high", "urgent")),
		mcp.WithString("column_id", mcp.Description("Move to this column")),
		mcp.WithString("assignee_id", mcp.Description("Member id, or empty string to unassign")),
		mcp.WithString("due_date", mcp.Description("RFC3339 date or YYYY-MM-DD; empty string clears it")),
		mcp.WithNumber("estimate", mcp.Description("Estimate in points")),
		mcp.WithArray("labels", mcp.Description("Replace labels with these ids"), mcp.WithStringItems()),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		patch := map[string]any{}
		args := req.GetArguments()
		for _, key := range []string{"title", "description", "priority", "column_id", "due_date"} {
			if v, ok := args[key]; ok {
				patch[key] = v
			}
		}
		if v, ok := args["assignee_id"]; ok {
			s := fmt.Sprint(v)
			if s == "" || s == "<nil>" {
				patch["assignee_id"] = nil
			} else {
				patch["assignee_id"] = s
			}
		}
		if v, ok := args["estimate"]; ok {
			patch["estimate"] = v
		}
		if v, ok := args["labels"]; ok {
			patch["label_ids"] = v
		}
		t, err := c.UpdateTask(id, patch)
		if err != nil {
			return fail(err)
		}
		return jsonResult(t)
	})

	add(s, mcp.NewTool("move_task",
		mcp.WithDescription("Move a task to another column, or reorder it before another task."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
		mcp.WithString("column_id", mcp.Description("Target column id")),
		mcp.WithString("before_id", mcp.Description("Place before this task id")),
		mcp.WithNumber("position", mcp.Description("Exact fractional position (alternative to before_id)")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		var pos *float64
		if v, ok := req.GetArguments()["position"]; ok {
			if f, ok := v.(float64); ok {
				pos = &f
			}
		}
		t, err := c.MoveTask(id, req.GetString("column_id", ""), req.GetString("before_id", ""), pos)
		if err != nil {
			return fail(err)
		}
		return jsonResult(t)
	})

	add(s, mcp.NewTool("delete_task",
		mcp.WithDescription("Delete a task permanently."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		if err := c.DeleteTask(id); err != nil {
			return fail(err)
		}
		return mcp.NewToolResultText("deleted " + id), nil
	})

	add(s, mcp.NewTool("search_tasks",
		mcp.WithDescription("Search tasks by text across all projects or one project."),
		mcp.WithString("q", mcp.Required(), mcp.Description("Search query")),
		projectArg(),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("q")
		if err != nil {
			return fail(err)
		}
		tasks, err := c.Search(req.GetString("project_id", ""), q)
		if err != nil {
			return fail(err)
		}
		return jsonResult(tasks)
	})

	add(s, mcp.NewTool("list_labels",
		mcp.WithDescription("List labels available in a project."),
		projectArg(),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		labels, err := c.ListLabels(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(labels)
	})

	add(s, mcp.NewTool("create_label",
		mcp.WithDescription("Create a label in a project."),
		projectArg(),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Description("Hex color")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		name, err := req.RequireString("name")
		if err != nil {
			return fail(err)
		}
		l, err := c.CreateLabel(id, name, req.GetString("color", ""))
		if err != nil {
			return fail(err)
		}
		return jsonResult(l)
	})

	add(s, mcp.NewTool("list_members",
		mcp.WithDescription("List the team members who can be assigned to tasks."),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		members, err := c.ListMembers()
		if err != nil {
			return fail(err)
		}
		return jsonResult(members)
	})

	add(s, mcp.NewTool("create_member",
		mcp.WithDescription("Add a member to the team."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Member name")),
		mcp.WithString("email", mcp.Description("Email")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return fail(err)
		}
		m, err := c.CreateMember(name, req.GetString("email", ""), "")
		if err != nil {
			return fail(err)
		}
		return jsonResult(m)
	})

	add(s, mcp.NewTool("add_subtask",
		mcp.WithDescription("Add a checklist item to a task."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Subtask title")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		title, err := req.RequireString("title")
		if err != nil {
			return fail(err)
		}
		if err := c.AddSubtask(id, title); err != nil {
			return fail(err)
		}
		t, err := c.GetTask(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(t)
	})

	add(s, mcp.NewTool("toggle_subtask",
		mcp.WithDescription("Mark a checklist item done or not done."),
		mcp.WithString("subtask_id", mcp.Required(), mcp.Description("Subtask id (from get_task)")),
		mcp.WithBoolean("done", mcp.Description("Desired state (default true)")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("subtask_id")
		if err != nil {
			return fail(err)
		}
		if err := c.UpdateSubtask(id, map[string]any{"done": req.GetBool("done", true)}); err != nil {
			return fail(err)
		}
		return mcp.NewToolResultText("updated " + id), nil
	})

	add(s, mcp.NewTool("add_comment",
		mcp.WithDescription("Leave a comment on a task."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Comment text")),
		mcp.WithString("author", mcp.Description("Author name")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		body, err := req.RequireString("body")
		if err != nil {
			return fail(err)
		}
		if err := c.AddComment(id, req.GetString("author", ""), body); err != nil {
			return fail(err)
		}
		return mcp.NewToolResultText("comment added"), nil
	})

	add(s, mcp.NewTool("list_comments",
		mcp.WithDescription("List comments on a task."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task id or reference")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		comments, err := c.ListComments(id)
		if err != nil {
			return fail(err)
		}
		return jsonResult(comments)
	})

	add(s, mcp.NewTool("get_activity",
		mcp.WithDescription("Get the recent activity feed of a project."),
		projectArg(),
		mcp.WithNumber("limit", mcp.Description("Max entries (default 30)")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := resolveProject(c, req)
		if err != nil {
			return fail(err)
		}
		activity, err := c.ListActivity(id, req.GetInt("limit", 30))
		if err != nil {
			return fail(err)
		}
		return jsonResult(activity)
	})

	add(s, mcp.NewTool("list_webhooks",
		mcp.WithDescription("List registered webhooks that receive board events."),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		webhooks, err := c.ListWebhooks()
		if err != nil {
			return fail(err)
		}
		return jsonResult(webhooks)
	})

	add(s, mcp.NewTool("create_webhook",
		mcp.WithDescription("Register a webhook URL that receives board events (task.created, task.updated, task.moved, task.deleted, comment.added) as signed HTTP POSTs. Use this to wake agents when work appears."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Endpoint that receives POSTs")),
		mcp.WithString("project", mcp.Description("Project id or key; empty means all projects")),
		mcp.WithString("events", mcp.Description("Comma-separated event types, default *")),
		mcp.WithString("secret", mcp.Description("HMAC secret for X-Glplan-Signature verification")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := req.RequireString("url")
		if err != nil {
			return fail(err)
		}
		in := api.WebhookInput{URL: url, Secret: req.GetString("secret", ""), Events: req.GetString("events", "")}
		if proj := req.GetString("project", ""); proj != "" {
			id, err := resolveProject(c, req)
			if err != nil {
				return fail(err)
			}
			in.ProjectID = id
		}
		wh, err := c.CreateWebhook(in)
		if err != nil {
			return fail(err)
		}
		return jsonResult(wh)
	})

	add(s, mcp.NewTool("delete_webhook",
		mcp.WithDescription("Remove a webhook registration."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Webhook id")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		if err := c.DeleteWebhook(id); err != nil {
			return fail(err)
		}
		return mcp.NewToolResultText("webhook deleted"), nil
	})

	add(s, mcp.NewTool("test_webhook",
		mcp.WithDescription("Send a webhook.test event to a webhook endpoint to verify delivery."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Webhook id")),
	), func(req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return fail(err)
		}
		if err := c.TestWebhook(id); err != nil {
			return fail(err)
		}
		return mcp.NewToolResultText("test event delivered"), nil
	})
}

func registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("plan_sprint",
		mcp.WithPromptDescription("Plan a sprint from the current glplan board"),
		mcp.WithArgument("project_id", mcp.ArgumentDescription("Project id or key")),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		project := req.Params.Arguments["project_id"]
		if project == "" {
			project = "(first project)"
		}
		prompt := fmt.Sprintf(`You are planning a sprint with the glplan MCP server.
1. Call get_board for project %s to see columns and counts.
2. Call list_tasks with include_done=false to see pending work.
3. Group tasks by priority and theme, estimate capacity, and propose a sprint goal.
4. Use create_task to break down anything too large, and move_task to pull selected tasks into "In Progress".
5. Summarize the plan with task references and owners.`, project)
		return mcp.NewGetPromptResult("Plan a sprint",
			[]mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt))}), nil
	})

	s.AddPrompt(mcp.NewPrompt("triage",
		mcp.WithPromptDescription("Triage pending tasks and clean up the board"),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		prompt := `Triage the glplan board:
1. Call list_tasks with include_done=false.
2. Look for vague titles, missing priorities, overdue tasks and unlabeled items.
3. For each issue, use update_task to add a priority, labels or a due date, and add_comment to explain the change.
4. Report what you changed with task references.`
		return mcp.NewGetPromptResult("Triage tasks",
			[]mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(strings.TrimSpace(prompt)))}), nil
	})
}
