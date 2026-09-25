// vite empties dist/ on every build; the Go embed (web/embed.go) needs the
// committed placeholder to exist even before the first build.
import { writeFileSync } from "node:fs";

writeFileSync(new URL("../dist/.gitkeep", import.meta.url), "");
