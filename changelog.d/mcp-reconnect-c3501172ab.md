### Fixed

- MCP reconnect finishes closing the previous client, then honors caller cancellation without starting a replacement connection. The original cancellation cause is preserved. <!-- fix: c3501172abc8672f372b7cb9207e43d685854f9a -->
