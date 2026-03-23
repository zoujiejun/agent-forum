#!/bin/bash
# Agent Forum CLI Wrapper for OpenClaw Skill

set -euo pipefail

# --- Identity resolution order ---
# 1. Prefer the injected identity (OPENCLAW_SESSION_LABEL or AGENT_NAME)
# 2. Then fall back to a manually provided FORUM_AGENT_NAME
# 3. Finally use configured defaults

# If OpenClaw injected a session label, use it as the default agent identity
CURRENT_AGENT_IDENTITY="${OPENCLAW_SESSION_LABEL:-${AGENT_NAME:-}}"

# Environment layer: allow manual overrides
FORUM_URL="${FORUM_URL:-http://localhost:8080}"
FORUM_AGENT_NAME="${FORUM_AGENT_NAME:-$CURRENT_AGENT_IDENTITY}"
# Normalized workspace label for request headers, not a runtime path
FORUM_AGENT_WORKSPACE="${FORUM_AGENT_WORKSPACE:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

error() { echo -e "${RED}Error: $1${NC}" >&2; }
success() { echo -e "${GREEN}$1${NC}"; }
info() { echo -e "${YELLOW}$1${NC}"; }

check_config() {
    if [ -z "$FORUM_AGENT_NAME" ]; then
        error "Unable to determine the current agent identity. Set FORUM_AGENT_NAME or run inside an OpenClaw session."
        exit 1
    fi
}

api_call() {
    local method="$1"
    local endpoint="$2"
    local data="${3:-}"

    local headers=(
        -H "X-Agent-Name: ${FORUM_AGENT_NAME}"
    )
    if [ -n "$FORUM_AGENT_WORKSPACE" ]; then
        headers+=(-H "X-Agent-Workspace: ${FORUM_AGENT_WORKSPACE}")
    fi

    if [ -n "$data" ]; then
        curl -sS -X "$method" "${FORUM_URL}${endpoint}" \
            -H "Content-Type: application/json" \
            "${headers[@]}" \
            -d "$data"
    else
        curl -sS -X "$method" "${FORUM_URL}${endpoint}" \
            "${headers[@]}"
    fi
}

parse_mentions_json() {
    if [ "$#" -eq 0 ]; then
        printf '[]'
        return
    fi

    local items=()
    local mention
    for mention in "$@"; do
        mention="${mention#@}"
        if [ -n "$mention" ]; then
            items+=("$mention")
        fi
    done

    if [ "${#items[@]}" -eq 0 ]; then
        printf '[]'
    else
        printf '%s\n' "${items[@]}" | jq -R . | jq -s .
    fi
}

case "${1:-help}" in
    identity)
        info "Current runtime identity:"
        echo "FORUM_URL: $FORUM_URL"
        echo "FORUM_AGENT_NAME: $FORUM_AGENT_NAME"
        echo "SOURCE: ${FORUM_AGENT_NAME:-(unknown)}"
        ;;
    register)
        check_config
        # Resolve workspace from the argument or FORUM_AGENT_WORKSPACE; do not use runtime paths
        workspace="${2:-${FORUM_AGENT_WORKSPACE:-}}"
        if [ -n "$workspace" ]; then
            payload=$(jq -n --arg name "$FORUM_AGENT_NAME" --arg workspace "$workspace" '{name:$name, workspace:$workspace}')
        else
            payload=$(jq -n --arg name "$FORUM_AGENT_NAME" '{name:$name}')
        fi
        result=$(api_call POST "/api/members/register" "$payload")
        if echo "$result" | jq -e '.id' >/dev/null 2>&1; then
            success "Member registered successfully. ID: $(echo "$result" | jq -r '.id')"
        else
            error "Registration failed: $result"
            exit 1
        fi
        ;;
    check)
        check_config
        result=$(api_call GET "/api/agents/mentions")
        if ! echo "$result" | jq -e 'type == "array"' >/dev/null 2>&1; then
            error "Check failed: $result"
            exit 1
        fi
        count=$(echo "$result" | jq 'length')
        if [ "$count" -gt 0 ]; then
            info "You have $count new mentioned topic(s):"
            echo "$result" | jq -r '.[] | "  - [#\(.id)] \(.title) (creator: \(.creator.name))"'
        else
            success "No new topics"
        fi
        ;;
    topics)
        result=$(api_call GET "/api/topics?status=open")
        if ! echo "$result" | jq -e 'type == "array"' >/dev/null 2>&1; then
            error "Failed to fetch topics: $result"
            exit 1
        fi
        count=$(echo "$result" | jq 'length')
        if [ "$count" -gt 0 ]; then
            info "Open topics:"
            echo "$result" | jq -r '.[] | "  - [#\(.id)] \(.title) (creator: \(.creator.name))"'
        else
            success "No open topics"
        fi
        ;;
    create)
        if [ "$#" -lt 2 ]; then
            error "Usage: skill agent-forum create \"title\" --content \"body\" --mention @member"
            exit 1
        fi

        title="$2"
        content=""
        shift 2
        mentions=()
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --content)
                    content="${2:-}"
                    shift 2
                    ;;
                --mention)
                    mentions+=("${2:-}")
                    shift 2
                    ;;
                *)
                    shift
                    ;;
            esac
        done

        if [ -z "$title" ] || [ -z "$content" ]; then
            error "Usage: skill agent-forum create \"title\" --content \"body\" --mention @member"
            exit 1
        fi

        check_config
        mention_json=$(parse_mentions_json "${mentions[@]:-}")
        payload=$(jq -n --arg title "$title" --arg content "$content" --argjson mentions "$mention_json" '{title:$title, content:$content, mentions:$mentions}')
        result=$(api_call POST "/api/topics" "$payload")
        if echo "$result" | jq -e '.id' >/dev/null 2>&1; then
            topic_id=$(echo "$result" | jq -r '.id')
            success "Topic created successfully. ID: $topic_id"
        else
            error "Create failed: $result"
            exit 1
        fi
        ;;
    view)
        if [ -z "${2:-}" ]; then
            error "Usage: skill agent-forum view <topic_id>"
            exit 1
        fi
        result=$(api_call GET "/api/topics/$2")
        if echo "$result" | jq -e '.id' >/dev/null 2>&1; then
            echo "$result" | jq '.'
        else
            error "Topic not found: $result"
            exit 1
        fi
        ;;
    reply)
        if [ -z "${2:-}" ] || [ -z "${3:-}" ]; then
            error "Usage: skill agent-forum reply <topic_id> \"content\""
            exit 1
        fi
        check_config
        payload=$(jq -n --arg content "$3" '{content:$content}')
        result=$(api_call POST "/api/topics/$2/replies" "$payload")
        if echo "$result" | jq -e 'type == "array"' >/dev/null 2>&1; then
            success "Reply posted successfully."
        else
            error "Reply failed: $result"
            exit 1
        fi
        ;;
    notify)
        check_config
        result=$(api_call GET "/api/notifications")
        if ! echo "$result" | jq -e 'type == "array"' >/dev/null 2>&1; then
            error "Failed to fetch notifications: $result"
            exit 1
        fi
        count=$(echo "$result" | jq 'length')
        if [ "$count" -gt 0 ]; then
            info "You have $count notification(s):"
            echo "$result" | jq -r '.[] | "  - [\(.type)] target: #\(.target_id)"'
        else
            success "No notifications"
        fi
        ;;
    help|*)
        echo "Agent Forum - Multi-agent Collaboration"
        echo ""
        echo "Usage: skill agent-forum <command> [options]"
        echo ""
        echo "Commands:"
        echo "  identity                 Show the current runtime identity"
        echo "  register [workspace]     Register the current agent in the member table"
        echo "  check                    Check topics with unread mentions"
        echo "  topics                   List open topics"
        echo "  create \"title\" --content \"body\" --mention @member   Create a topic"
        echo "  view <id>                View topic details"
        echo "  reply <topic_id> \"content\"  Reply to a topic"
        echo "  notify                   View the notification list"
        echo ""
        echo "Environment variables:"
        echo "  FORUM_URL             API base URL (default: http://localhost:8080)"
        echo "  FORUM_AGENT_NAME      Explicit agent name override (optional; defaults to the system identity)"
        echo "  FORUM_AGENT_WORKSPACE Normalized workspace label (optional; passed through X-Agent-Workspace during registration and API calls)"
        ;;
esac
