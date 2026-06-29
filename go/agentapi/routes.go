package agentapi

type Route struct {
	Symbol string
	Method string
	Path   string
}

func SupportedRoutes() []Route {
	return []Route{
		{"responses.create", "POST", "/v1/responses"},
		{"responses.list", "GET", "/v1/responses"},
		{"responses.retrieve", "GET", "/v1/responses/{response_id}"},
		{"responses.cancel", "POST", "/v1/responses/{response_id}/cancel"},
		{"responses.children", "GET", "/v1/responses/{response_id}/children"},
		{"responses.events", "GET", "/v1/responses/{response_id}/events"},
		{"responses.volume", "GET", "/v1/responses/{response_id}/volume"},
		{"agent.create", "POST", "/v1/agent"},
		{"memories.search", "POST", "/v1/memories/search"},
		{"models.list", "GET", "/v1/models"},
		{"presets.list", "GET", "/v1/presets"},
		{"tools.list", "GET", "/v1/tools"},
		{"volumes.list", "GET", "/v1/volumes"},
		{"volumes.create", "POST", "/v1/volumes"},
		{"volumes.retrieve", "GET", "/v1/volumes/{volume_id}"},
		{"volumes.update", "PATCH", "/v1/volumes/{volume_id}"},
		{"volumes.delete", "DELETE", "/v1/volumes/{volume_id}"},
		{"volumes.reconcile_usage", "POST", "/v1/volumes/{volume_id}/usage/reconcile"},
		{"volumes.entries", "GET", "/v1/volumes/{volume_id}/entries"},
		{"volumes.search", "GET", "/v1/volumes/{volume_id}/search"},
		{"volumes.read_file", "GET", "/v1/volumes/{volume_id}/files/{path}"},
		{"volumes.write_file", "PUT", "/v1/volumes/{volume_id}/files/{path}"},
		{"volumes.delete_path", "DELETE", "/v1/volumes/{volume_id}/paths/{path}"},
		{"volumes.create_directory", "POST", "/v1/volumes/{volume_id}/directories"},
		{"volumes.archive", "GET", "/v1/volumes/{volume_id}/archive"},
		{"volumes.summarize", "POST", "/v1/volumes/{volume_id}/summarize"},
		{"volumes.grep", "GET", "/v1/volumes/{volume_id}/grep"},
		{"volumes.read_lines", "GET", "/v1/volumes/{volume_id}/file_lines/{path}"},
		{"volumes.patch_lines", "PATCH", "/v1/volumes/{volume_id}/file_lines/{path}"},
		{"skills.list", "GET", "/v1/skills"},
		{"skills.create", "POST", "/v1/skills"},
		{"skills.discover", "POST", "/v1/skills/discover"},
		{"skills.focus", "POST", "/v1/skills/focus"},
		{"skills.create_dev", "POST", "/v1/skills/create_dev"},
		{"skills.update_file", "POST", "/v1/skills/update_file"},
		{"skills.retrieve", "GET", "/v1/skills/{skill_id}"},
		{"skills.update", "PATCH", "/v1/skills/{skill_id}"},
		{"skills.archive", "POST", "/v1/skills/{skill_id}/archive"},
		{"skills.delete", "DELETE", "/v1/skills/{skill_id}"},
		{"skills.accept_dev", "POST", "/v1/skills/{skill_id}/accept_dev"},
		{"skills.discard_dev", "POST", "/v1/skills/{skill_id}/discard_dev"},
		{"skills.files", "GET", "/v1/skills/{skill_id}/files"},
		{"skills.read_file", "GET", "/v1/skills/{skill_id}/files/{path}"},
		{"skills.write_file", "PUT", "/v1/skills/{skill_id}/files/{path}"},
		{"skills.delete_file", "DELETE", "/v1/skills/{skill_id}/files/{path}"},
		{"skills.export", "GET", "/v1/skills/{skill_id}/export"},
		{"skills.import", "POST", "/v1/skills/{skill_id}/import"},
		{"skills.diff", "GET", "/v1/skills/{skill_id}/diff"},
		{"auth.device_start", "POST", "/v1/auth/device/start"},
		{"auth.device_poll", "POST", "/v1/auth/device/poll"},
	}
}
