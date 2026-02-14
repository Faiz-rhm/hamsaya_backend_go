# Poll options update (PULL post edit)

Editing a PULL post's options (add/remove) is applied in `UpdatePost` when the request includes `poll_options` with 2+ items.

**You must run the newly built server for the fix to apply.**

1. Stop the current backend process (Ctrl+C or kill the process on port 8080).
2. From this directory run:
   ```bash
   make build
   ./bin/server
   ```
   Or in one step: `make run` (rebuilds and runs).
3. In the app, edit a PULL post, change options (e.g. to A, B only), save. GET `/posts/:id/polls` should then return only the new options.

**Logs to expect when it works:**  
`UpdatePost request` with `poll_options_count: 2`, then `Updating poll options for PULL post`, then `Poll options updated`.
