---
name: mcp-advisor
description: "Evaluates the need to use or build MCP servers for secure system integration."
---

# When to suggest an MCP instead of custom code

Answer in PT-BR, objective (global AGENTS.md). The goal here is to **evaluate**, not to push MCP for everything — many simple, one-off integrations remain cheaper as direct code.

## Signals that an MCP is worth it

- The integration is **repeated** across the project/sessions (the same API being called in several places), not a single, isolated call.
- The external system changes often (live data: production, tickets, documents, dashboards) and the agent would need to "re-query" in future conversations — custom code does not persist that access between sessions, an MCP does.
- A known, maintained MCP already exists for that system (e.g.: database, Slack, GitHub, cloud tools) — reinventing a custom client is rework and extra bug surface.
- The task needs authenticated, recurring action on a third-party service (not just an occasional public GET).

## Signals that it is NOT worth it (stay with direct code)

- It is a single API call, within the scope of a specific feature of the user's own product (e.g.: the client's app calling its own payment API) — that is product code, not agent tooling.
- No MCP exists for the system in question and creating one from scratch would be disproportionate to the value (a one-off automation does not justify maintaining an MCP server).
- The access is simple, stable and does not need to be reused between conversations (e.g.: reading a local CSV file once).

## How to conduct the evaluation

1. Identify the external system involved and whether it is something that will reappear in other tasks.
2. Confirm whether a relevant MCP already exists — ask the user which MCPs they already have configured (`ListAgents`/config do not show this directly; ask or check `.claude/settings.json`/the project's MCP config) before suggesting installing a new one.
3. If there is no MCP and the integration is recurring, present the option as a short suggestion — do not decide on your own to install/configure anything without approval (a configuration change counts as an action that needs confirmation, per AGENTS.md).
4. If it is the user's own product code (not agent tooling), do not suggest an MCP — that is application scope, not assistant scope.

## Discovery and security before installing

If the decision is to use an MCP, evaluate the origin before configuring anything:

- **Discovery**: the official catalog is the MCP Registry (published via `server.json` + the `mcp-publisher` CLI); the namespace reveals ownership (`io.github.<usuario>/...` requires GitHub login; your own domain requires a DNS/HTTP challenge). Prefer the server maintained by whoever maintains the system over a third-party fork.
- **Maintenance**: check the last push, open issues and license — an abandoned server becomes a risk surface in the agent's tooling.
- **Scan before installing**: there are scanners that cover agents, MCP servers and skills (e.g.: `snyk/agent-scan`); treat a third-party skill/server as unaudited code.
- **Isolated execution**: never run an untrusted MCP server directly on the user's filesystem/credentials — use a sandbox, container or microVM with restricted capabilities.
- **Scale**: with several servers/MCPs in the team, evaluate an aggregation gateway (centralized auth, rate limit, observability) instead of spreading N configurations across CLIs — it is only worth it when the number of servers justifies the extra process.

## Suggestion format

Be direct: the system name, why an MCP would help more than custom code, and ask whether the user wants you to configure/verify it — do not implement the custom integration "anyhow" and then mention the MCP as a footnote.
