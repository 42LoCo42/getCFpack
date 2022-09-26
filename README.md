# getCFpack
This tool is a simple downloader for CurseForge modpacks,
created after MultiMC stopped supporting the new modpack format.

To use it, obtain an [API Key](https://console.curseforge.com/#/api-keys) and put it into the file `key`.
To download a modpack, get its project ID
(for example, [Nomifactory](https://www.curseforge.com/minecraft/modpacks/nomifactory) has the ID 563950)
and execute `go run . <ID>` (e.g. `go run . 563950`).

## Possible TODOs
- create a MultiMC instance
- support for multiple pack versions (currently, only the first result is chosen)
- file integrity checks
- in-place updates
