package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	noSeed := fs.Bool("no-seed", false, "do not create sample data")
	showVersion := fs.Bool("version", false, "print version")
	_ = fs.Parse(args)
	if *showVersion {
		fmt.Println(version)
		return
	}

	switch cmd {
	case "tui", "board", "":
		runTUI(*server, *token)
	case "serve", "server":
		runServe(*data, *addr, *token, *noSeed)
	case "mcp", "mcp-server":
		if err := mcp.Run(*server, *token, version); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			os.Exit(1)
		}
	case "init":
		runInit(*data, *noSeed)
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
  init     cria o banco e dados de exemplo
  config   salva configuração em ~/.config/glplan/config.json
  version  mostra a versão

Flags:
  --server URL   endpoint da API (ou GLPLAN_API_URL)
  --token TOK    bearer token (ou GLPLAN_TOKEN)
  --data PATH    arquivo SQLite (ou GLPLAN_DATA)
  --addr ADDR    endereço do servidor, ex: :9876
  --no-seed      não cria dados de exemplo`)
}

func openStore(path string, seed bool) (*store.Store, error) {
	s, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	if seed {
		if err := s.Seed(); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

func runTUI(server, token string) {
	client := api.NewClient(server, token)
	if err := client.Health(); err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
		fmt.Fprintln(os.Stderr, "iniciando 'glplan serve' em outro terminal para usar a interface.")
	}
	p := tea.NewProgram(tui.New(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}

func runServe(path, addr, token string, noSeed bool) {
	s, err := openStore(path, !noSeed)
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

func runInit(path string, noSeed bool) {
	s, err := openStore(path, !noSeed)
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
