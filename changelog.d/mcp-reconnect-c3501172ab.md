### Fixed

- Fixed incorrect MCP reconnect behavior after cancellation during transport cleanup. Previously, reconnect still started a replacement connection after the caller had canceled. Reconnect now completes cleanup and returns the original cancellation cause before starting a replacement. <!-- fix: c3501172abc8672f372b7cb9207e43d685854f9a -->
