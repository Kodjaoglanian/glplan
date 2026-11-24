#!/usr/bin/env bash
# Glplan installer — one-line install:
#   curl -fsSL https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.sh | bash
set -euo pipefail

REPO="Kodjaoglanian/glplan"
INSTALL_DIR="${GLPLAN_INSTALL_DIR:-$HOME/.glplan/bin}"
CONFIG_DIR="${GLPLAN_CONFIG_DIR:-$HOME/.glplan}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "${CYAN}[info]${NC} $*"; }
ok()      { echo -e "${GREEN}[ok]${NC}   $*"; }
warn()    { echo -e "${YELLOW}[warn]${NC} $*"; }
error()   { echo -e "${RED}[error]${NC} $*"; exit 1; }
tmp=""

detect_target() {
    local os arch
    os="$(uname -s)"
    arch="$(uname -m)"
    case "$os" in
        Linux) os="linux" ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *) error "Sistema não suportado: $os (builds disponíveis: linux, windows)" ;;
    esac
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) error "Arquitetura não suportada: $arch" ;;
    esac
    echo "${os}-${arch}"
}

fetch() {
    local url="$1" out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$out" || return 1
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$out" "$url" || return 1
    else
        error "curl ou wget é necessário para instalar"
    fi
}

latest_version() {
    if [ -n "${GLPLAN_VERSION:-}" ]; then
        echo "$GLPLAN_VERSION"
        return
    fi
    local api="https://api.github.com/repos/${REPO}/releases/latest"
    local resp
    resp="$(curl -fsSL "$api" 2>/dev/null || wget -qO- "$api" 2>/dev/null)" || \
        error "não foi possível consultar a última release"
    if command -v jq >/dev/null 2>&1; then
        echo "$resp" | jq -r '.tag_name'
    else
        echo "$resp" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
    fi
}

main() {
    echo ""
    echo -e "${CYAN}glplan — board kanban AI-first no terminal${NC}"
    echo ""

    local target version ext archive url tmp
    target="$(detect_target)"
    info "Alvo: $target"

    version="$(latest_version)"
    [ -n "$version" ] || error "versão vazia; use GLPLAN_VERSION=vX.Y.Z"
    info "Versão: $version"

    case "$target" in
        windows-*) ext="zip" ;;
        *)         ext="tar.gz" ;;
    esac
    archive="glplan-${version}-${target}.${ext}"
    url="https://github.com/${REPO}/releases/download/${version}/${archive}"
    info "Baixando $archive"

    tmp="$(mktemp -d)"
    trap 'rm -rf "${tmp:-}"' EXIT
    fetch "$url" "$tmp/$archive" || error "download falhou: $url"

    info "Verificando SHA256"
    if fetch "${url}.sha256" "$tmp/${archive}.sha256"; then
        if command -v sha256sum >/dev/null 2>&1; then
            (cd "$tmp" && sha256sum -c "${archive}.sha256") || error "checksum inválido"
        elif command -v shasum >/dev/null 2>&1; then
            (cd "$tmp" && shasum -a 256 -c "${archive}.sha256") || error "checksum inválido"
        else
            warn "sha256sum/shasum indisponível; pulando verificação"
        fi
        ok "Checksum verificado"
    else
        warn "arquivo .sha256 não encontrado; pulando verificação"
    fi

    info "Extraindo"
    case "$ext" in
        tar.gz) tar xzf "$tmp/$archive" -C "$tmp" ;;
        zip)
            if command -v unzip >/dev/null 2>&1; then
                unzip -oq "$tmp/$archive" -d "$tmp"
            else
                error "unzip é necessário para instalar no Windows via bash"
            fi
            ;;
    esac

    local binary="glplan"
    case "$target" in windows-*) binary="glplan.exe" ;; esac
    if [ ! -f "$tmp/$binary" ]; then
        error "binário $binary não encontrado no arquivo: $(ls "$tmp" | tr '\n' ' ')"
    fi

    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
    mv "$tmp/$binary" "$INSTALL_DIR/$binary"
    chmod +x "$INSTALL_DIR/$binary" 2>/dev/null || true
    ok "Instalado em $INSTALL_DIR/$binary"

    local shell_name rc path_line
    shell_name="$(basename "${SHELL:-bash}")"
    case "$shell_name" in
        bash)  rc="$HOME/.bashrc" ;;
        zsh)   rc="$HOME/.zshrc" ;;
        fish)  rc="$HOME/.config/fish/config.fish" ;;
        *)     rc="$HOME/.profile" ;;
    esac
    if [ "$shell_name" = "fish" ]; then
        path_line="set -gx PATH $INSTALL_DIR \$PATH"
    else
        path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
    if [ -f "$rc" ] && grep -Fq "$INSTALL_DIR" "$rc"; then
        info "PATH já configurado em $rc"
    else
        mkdir -p "$(dirname "$rc")"
        printf '\n# glplan\n%s\n' "$path_line" >> "$rc"
        ok "PATH adicionado em $rc"
    fi

    echo ""
    if "$INSTALL_DIR/$binary" --version >/dev/null 2>&1; then
        ok "Glplan $version instalado com sucesso"
        echo ""
        echo -e "  ${CYAN}glplan serve${NC}   inicia a API (um terminal)"
        echo -e "  ${CYAN}glplan tui${NC}     abre o board (outro terminal)"
        echo -e "  ${CYAN}glplan mcp${NC}     servidor MCP para agentes"
        echo ""
        warn "reinicie o terminal ou rode: source $rc"
    else
        warn "instalação concluída, mas a verificação do binário falhou"
    fi
}

main "$@"
