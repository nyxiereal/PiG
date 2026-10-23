### Fixed

- Go embedding hosts can bind extension appends through `coding.Session.AppendExtensionEntry`, retaining subprocess IDs and timestamps through the native Session writer and preserving `entry_appended` semantics. This prevents duplicate entries in the Node session mirror when a host previously had to allocate a second identity. <!-- fix: bf453203b656bcbb1aa0a0c6f70fa9cc1562ac5e -->
