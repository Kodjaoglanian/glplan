package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"glplan/internal/api"
	"glplan/internal/mcp"
	"glplan/internal/store"
	"glplan/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

type config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
	Data   string `json:"data"`
	Addr   string `json:"addr"`
}

func defaultDataPath() string {
	if v := os.Getenv("GLPLAN_DATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "glplan", "glplan.db")
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "glplan", "config.json")
}

func loadConfig() config {
	c := config{
		Server: os.Getenv("GLPLAN_API_URL"),
		Token:  os.Getenv("GLPLAN_TOKEN"),
		Data:   defaultDataPath(),
		Addr:   os.Getenv("GLPLAN_ADDR"),
	}
	b, err := os.ReadFile(configPath())
	if err != nil {
		return c
	}
	var file config
	if json.Unmarshal(b, &file) != nil {
		return c
	}
	if c.Server == "" {
		c.Server = file.Server
	}
	if c.Token == "" {
		c.Token = file.Token
	}
	if os.Getenv("GLPLAN_DATA") == "" && file.Data != "" {
		c.Data = file.Data
	}
	if c.Addr == "" {
		c.Addr = file.Addr
	}
	return c
}

func main() {
	cfg := loadConfig()
	args := os.Args[1:]
	cmd := "tui"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("glplan "+cmd, flag.ExitOnError)
	server := fs.String("server", cfg.Server, "API base URL")
	token := fs.String("token", cfg.Token, "bearer token")
	data := fs.String("data", cfg.Data, "SQLite database path")
	addr := fs.String("addr", cfg.Addr, "listen address for serve")
	demo := fs.Bool("demo", false, "create demo board with sample data")
	_ = fs.Bool("no-seed", false, "deprecated: demo data is not created by default")
	showVersion := fs.Bool("version", false, "print version")
	_ = fs.Parse(args)
	if *showVersion {
		fmt.Println(version)
		return
	}

	switch cmd {
	case "tui", "board", "":
		runTUI(*server, *token, *data, *demo)
	case "serve", "server":
		runServe(*data, *addr, *token, *demo)
	case "mcp", "mcp-server":
		client, embedded, err := clientWithEmbedded(*server, *token, *data, *demo)
		if embedded != nil {
			defer embedded.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "aviso:", err)
		}
		endpoint := client.BaseURL
		if err := mcp.Run(endpoint, *token, version); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			os.Exit(1)
		}
	case "init":
		runInit(*data, *demo)
	case "update":
		runUpdate()
	case "version":
		fmt.Println(version)
	case "config":
		runConfig(*server, *token, *data, *addr)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "comando desconhecido:", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`glplan — board kanban AI-first no terminal

Uso:
  glplan [comando] [flags]

Comandos:
  tui      abre a interface terminal (padrão)
  serve    inicia a API HTTP
  mcp      inicia o servidor MCP via stdio
  init     cria o banco e um projeto limpo
  update   atualiza o binário para a última release
  config   salva configuração em ~/.config/glplan/config.json
  version  mostra a versão

Flags:
  --server URL   endpoint da API (ou GLPLAN_API_URL)
  --token TOK    bearer token (ou GLPLAN_TOKEN)
  --data PATH    arquivo SQLite (ou GLPLAN_DATA)
  --addr ADDR    endereço do servidor, ex: :9876
  --demo         cria um board de demonstração com dados de exemplo`)
}

func openStore(path string, demo bool) (*store.Store, error) {
	s, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	if err := s.Bootstrap(); err != nil {
		s.Close()
		return nil, err
	}
	if demo {
		if err := s.Seed(); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

type embeddedAPI struct {
	url    string
	server *http.Server
	store  *store.Store
}

func startEmbeddedAPI(data, token string, demo bool) (*embeddedAPI, error) {
	s, err := openStore(data, demo)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.Close()
		return nil, err
	}
	api.SetVersion(version)
	httpServer := &http.Server{Handler: api.New(s, token).Handler()}
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "api embutida:", err)
		}
	}()
	return &embeddedAPI{url: "http://" + listener.Addr().String(), server: httpServer, store: s}, nil
}

func (e *embeddedAPI) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = e.server.Shutdown(ctx)
	_ = e.store.Close()
}

func isLocalAPI(server string) bool {
	if strings.TrimSpace(server) == "" {
		return true
	}
	u, err := url.Parse(server)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func shouldStartEmbedded(server string, healthErr error) bool {
	if healthErr == nil || !isLocalAPI(server) {
		return false
	}
	var apiErr *api.APIError
	return !errors.As(healthErr, &apiErr)
}

func clientWithEmbedded(server, token, data string, demo bool) (*api.Client, *embeddedAPI, error) {
	client := api.NewClient(server, token)
	healthErr := client.Health()
	if healthErr == nil {
		return client, nil, nil
	}
	if !shouldStartEmbedded(server, healthErr) {
		return client, nil, healthErr
	}
	embedded, err := startEmbeddedAPI(data, token, demo)
	if err != nil {
		return client, nil, err
	}
	embeddedClient := api.NewClient(embedded.url, token)
	if err := embeddedClient.Health(); err != nil {
		embedded.Close()
		return client, nil, err
	}
	return embeddedClient, embedded, nil
}

func runTUI(server, token, data string, demo bool) {
	client, embedded, err := clientWithEmbedded(server, token, data, demo)
	if embedded != nil {
		defer embedded.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
		fmt.Fprintln(os.Stderr, "para compartilhar o board, rode 'glplan serve' em outro terminal.")
	}
	backend := "api local"
	if embedded != nil {
		backend = "local embutido"
	} else if server != "" && !isLocalAPI(server) {
		if u, err := url.Parse(server); err == nil && u.Hostname() != "" {
			backend = u.Hostname()
		} else {
			backend = "api remota"
		}
	}
	p := tea.NewProgram(tui.NewWithBackend(client, backend), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}

func runServe(path, addr, token string, demo bool) {
	s, err := openStore(path, demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "banco:", err)
		os.Exit(1)
	}
	defer s.Close()
	api.SetVersion(version)
	server := api.New(s, token)
	if addr == "" {
		addr = ":9876"
	}
	if err := server.Listen(addr); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func runInit(path string, demo bool) {
	s, err := openStore(path, demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "banco:", err)
		os.Exit(1)
	}
	defer s.Close()
	fmt.Println("banco pronto:", path)
	projects, _ := s.ListProjects()
	for _, p := range projects {
		fmt.Printf("  %s  %s\n", p.Key, p.Name)
	}
}

func runConfig(server, token, data, addr string) {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(config{Server: server, Token: token, Data: data, Addr: addr}, "", "  ")
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("config salva:", p)
}
