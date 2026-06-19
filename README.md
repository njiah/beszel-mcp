# MCP Server for Beszel

This is a local MCP server for self-hosted beszel on private network.
Clone this repository and run the following command:

```bash
go build -o beszel-mcp
```

You can monitor the server response with MCP inspector.

```bash
npx @modelcontextprotocol/inspector ./beszel-mcp
```

Add to your MCP configuration (e.g., in Claude Desktop or Agent Builder or N8N):

```json
{
  "mcpServers": {
    "beszel": {
      "command": "/absolute/path/to/beszel-mcp",
      "env": {
        "BESZEL_URL": "http://beszel.your-tailnet:8090",
        "BESZEL_EMAIL": "mcp-monitor@example.com",
        "BESZEL_PASSWORD": "your-password"
      }
    }
  }
}
```

### What it does

Pulls logs and system information from your Beszel instance.

### References

- [MCP Protocol](https://modelcontextprotocol.io/docs/develop/build-server#go)
