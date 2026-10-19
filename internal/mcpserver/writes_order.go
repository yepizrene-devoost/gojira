package mcpserver

import (
	"context"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The SDK lists tools alphabetically, not in registration order. Only opt-in
// servers reorder the single tool page, preserving the read-only wire contract.
// The fixed allowlist fits the SDK's default page size.
func orderWriteTools(server *mcp.Server) {
	rank := map[string]int{}
	for i, entry := range writeTools() {
		rank["gojira_"+entry.action] = i + 1
	}
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			value, err := next(ctx, method, req)
			if method == "tools/list" && err == nil {
				if list, ok := value.(*mcp.ListToolsResult); ok {
					sort.SliceStable(list.Tools, func(i, j int) bool { return rank[list.Tools[i].Name] < rank[list.Tools[j].Name] })
				}
			}
			return value, err
		}
	})
}
