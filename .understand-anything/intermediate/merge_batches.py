import json

with open('.understand-anything/knowledge-graph.json', encoding='utf-8') as f:
    graph = json.load(f)

existing_ids = {n['id'] for n in graph['nodes']}
existing_edge_keys = {(e['source'], e['target'], e['type']) for e in graph['edges']}

# ======== Batch 2: locales + settings_app.go ========
batch2_nodes = [
    {'id':'file:desktop/frontend/src/locales/en.ts','type':'file','name':'en.ts','summary':'Canonical English UI translation dictionary for Reasonix desktop frontend.','tags':['i18n','translations','frontend','typescript','english'],'complexity':'moderate'},
    {'id':'file:desktop/frontend/src/locales/zh-TW.ts','type':'file','name':'zh-TW.ts','summary':'Traditional Chinese UI translation file. Imports DictKey from en.ts and enforces complete key parity.','tags':['i18n','translations','frontend','typescript','chinese-traditional'],'complexity':'moderate'},
    {'id':'file:desktop/frontend/src/locales/zh.ts','type':'file','name':'zh.ts','summary':'Simplified Chinese UI translation file. Imports DictKey from en.ts and enforces complete key parity.','tags':['i18n','translations','frontend','typescript','chinese-simplified'],'complexity':'moderate'},
    {'id':'file:desktop/settings_app.go','type':'file','name':'settings_app.go','summary':'Desktop Settings panel command surface in Go. Reads resolved config, applies edits through internal/config/edit.go mutation API, and rebuilds controllers for live effect.','tags':['go','settings','desktop','config','panel','app'],'complexity':'complex'},
    {'id':'type:desktop/settings_app.go:SettingsView','type':'class','name':'SettingsView','summary':'Top-level Settings panel payload struct containing all desktop settings views.','tags':['go','struct','settings','payload'],'complexity':'moderate'},
    {'id':'type:desktop/settings_app.go:App','type':'class','name':'App','summary':'Main desktop app struct with methods for reading and writing all settings panels.','tags':['go','struct','app','settings'],'complexity':'complex'},
    {'id':'type:desktop/settings_app.go:ProviderView','type':'class','name':'ProviderView','summary':'View model for a single provider: name, kind, base URL, models, etc.','tags':['go','struct','provider','view'],'complexity':'moderate'},
    {'id':'type:desktop/settings_app.go:BotSettingsView','type':'class','name':'BotSettingsView','summary':'Aggregate bot settings view containing enabled state, model, tool approval mode, etc.','tags':['go','struct','bot','settings'],'complexity':'moderate'},
    {'id':'type:desktop/settings_app.go:DesktopStartupSettingsView','type':'class','name':'DesktopStartupSettingsView','summary':'Lightweight settings subset for frontend startup, excludes providers to avoid slow keychain resolution.','tags':['go','struct','settings','startup'],'complexity':'simple'},
]
batch2_edges = [
    {'source':'file:desktop/frontend/src/locales/zh-TW.ts','target':'file:desktop/frontend/src/locales/en.ts','type':'imports','direction':'forward','weight':1.0},
    {'source':'file:desktop/frontend/src/locales/zh.ts','target':'file:desktop/frontend/src/locales/en.ts','type':'imports','direction':'forward','weight':1.0},
]

# ======== Batch 3: effort.go, edit_test.go, effort_test.go ========
batch3_nodes = [
    {'id':'file:internal/config/effort.go','type':'file','name':'effort.go','summary':'Effort config logic for providers: effort capability detection, normalization, reasoning protocol resolution, and vendor-specific effort level mapping.','tags':['config','effort','reasoning-protocol','provider'],'complexity':'moderate'},
    {'id':'file:internal/config/edit_test.go','type':'file','name':'edit_test.go','summary':'Config edit tests covering effort, UI, desktop preferences, provider config, save operations, and normalization.','tags':['config','test','edit','effort','ui','desktop'],'complexity':'complex'},
    {'id':'file:internal/config/effort_test.go','type':'file','name':'effort_test.go','summary':'Effort config unit tests for MiniMax entry detection, capability levels, normalization, and garbage input rejection.','tags':['config','test','effort','minimax','deepseek'],'complexity':'moderate'},
    {'id':'function:internal/config/effort.go:EffortCapabilityForEntry','type':'function','name':'EffortCapabilityForEntry','summary':'Returns user-facing /effort levels based on reasoning protocol, model registry, or entry kind heuristics.','tags':['effort','capability','reasoning-protocol'],'complexity':'moderate'},
    {'id':'function:internal/config/effort.go:NormalizeEffort','type':'function','name':'NormalizeEffort','summary':'Maps a user-supplied /effort level into stored config value; handles vendor-specific level migration.','tags':['effort','normalization','provider'],'complexity':'moderate'},
    {'id':'function:internal/config/effort.go:EffortDisplay','type':'function','name':'EffortDisplay','summary':'Returns the selected /effort level display string, falling back to auto.','tags':['effort','display'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:EffectiveEffort','type':'function','name':'EffectiveEffort','summary':'Resolves the provider-visible effort value.','tags':['effort','effective'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:ReasoningProtocolForEntry','type':'function','name':'ReasoningProtocolForEntry','summary':'Resolves the reasoning protocol for a provider entry via config, registry, or legacy heuristics.','tags':['reasoning-protocol','provider'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:isDeepSeekEntry','type':'function','name':'isDeepSeekEntry','summary':'Reports whether a provider entry points to DeepSeek API.','tags':['deepseek','provider'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:isMiniMaxEntry','type':'function','name':'isMiniMaxEntry','summary':'Reports whether a provider entry points to MiniMax endpoint.','tags':['minimax','provider'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:deepSeekEffortCapability','type':'function','name':'deepSeekEffortCapability','summary':'Returns DeepSeek effort capability [auto, disabled, high, max] with default high.','tags':['effort','deepseek','capability'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:openAIEffortCapability','type':'function','name':'openAIEffortCapability','summary':'Returns OpenAI effort capability [auto, low, medium, high] with default auto.','tags':['effort','openai','capability'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:EffortCapability','type':'class','name':'EffortCapability','summary':'Struct describing abstract effort levels a provider/model can set.','tags':['struct','effort','capability'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:modelReasoningCapability','type':'class','name':'modelReasoningCapability','summary':'Internal struct mapping model names to reasoning protocol, levels, default effort.','tags':['struct','model','reasoning'],'complexity':'simple'},
    {'id':'concept:internal/config/effort.go:modelReasoningCapabilities','type':'concept','name':'modelReasoningCapabilities','summary':'Registry mapping known model names to their reasoning protocol, effort levels, defaults.','tags':['registry','model','reasoning','effort'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizeStoredEffort','type':'function','name':'normalizeStoredEffort','summary':'Normalizes effort value for storage.','tags':['effort','normalization'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizeEffortLevel','type':'function','name':'normalizeEffortLevel','summary':'Trims whitespace and lowercases effort level.','tags':['effort','normalization'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizeReasoningProtocol','type':'function','name':'normalizeReasoningProtocol','summary':'Normalize reasoning protocol string.','tags':['reasoning-protocol','normalization'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizedSupportedEfforts','type':'function','name':'normalizedSupportedEfforts','summary':'Deduplicates and normalizes supported effort levels.','tags':['effort','normalization','supported'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizedProviderHeaders','type':'function','name':'normalizedProviderHeaders','summary':'Trims whitespace from header key/value pairs.','tags':['provider','headers','normalization'],'complexity':'simple'},
    {'id':'function:internal/config/effort.go:normalizedModelOverrides','type':'function','name':'normalizedModelOverrides','summary':'Normalizes model override fields.','tags':['model','override','normalization'],'complexity':'moderate'},
]
batch3_edges = [
    {'source':'file:internal/config/effort.go','target':'file:internal/config/edit_test.go','type':'tested_by','direction':'forward','weight':0.8},
    {'source':'file:internal/config/effort.go','target':'file:internal/config/effort_test.go','type':'tested_by','direction':'forward','weight':0.9},
    {'source':'file:internal/config/edit_test.go','target':'file:internal/config/effort.go','type':'depends_on','direction':'forward','weight':0.8},
    {'source':'file:internal/config/effort_test.go','target':'file:internal/config/effort.go','type':'depends_on','direction':'forward','weight':0.9},
    {'source':'function:internal/config/effort.go:EffortCapabilityForEntry','target':'function:internal/config/effort.go:EffortCapability','type':'depends_on','direction':'forward','weight':0.9},
    {'source':'function:internal/config/effort.go:NormalizeEffort','target':'function:internal/config/effort.go:isMiniMaxEntry','type':'calls','direction':'forward','weight':0.6},
    {'source':'function:internal/config/effort.go:EffortCapabilityForEntry','target':'function:internal/config/effort.go:isMiniMaxEntry','type':'calls','direction':'forward','weight':0.6},
    {'source':'function:internal/config/effort.go:EffortCapabilityForEntry','target':'function:internal/config/effort.go:isDeepSeekEntry','type':'calls','direction':'forward','weight':0.5},
    {'source':'function:internal/config/effort.go:EffortCapabilityForEntry','target':'function:internal/config/effort.go:deepSeekEffortCapability','type':'calls','direction':'forward','weight':0.7},
    {'source':'function:internal/config/effort.go:EffortCapabilityForEntry','target':'concept:internal/config/effort.go:modelReasoningCapabilities','type':'depends_on','direction':'forward','weight':0.8},
]

# ======== Batch 4: openai, builtins, prefix ========
batch4_nodes = [
    {'id':'file:internal/provider/openai/openai.go','type':'file','name':'openai.go','summary':'OpenAI-compatible /chat/completions provider implementing provider.Provider. Handles DeepSeek, MiniMax M3, generic OpenAI backends.','tags':['provider','openai','llm','streaming','sse'],'complexity':'complex'},
    {'id':'file:internal/provider/openai/openai_test.go','type':'file','name':'openai_test.go','summary':'Tests for the OpenAI provider covering retry, auth, vendor thinking modes, effort validation, usage normalisation.','tags':['provider','openai','test','unit-test'],'complexity':'complex'},
    {'id':'file:internal/skill/builtins.go','type':'file','name':'builtins.go','summary':'Defines Reasonix built-in skills: init, explore, research, install-capability, review, security-review, test, understand.','tags':['skill','builtin','subagent','tool-registry'],'complexity':'moderate'},
    {'id':'file:internal/understand/prefix.go','type':'file','name':'prefix.go','summary':'Provides cache-stable system-prompt prefix injection from the Understand-Anything knowledge graph.','tags':['understand','knowledge-graph','system-prompt','prefix'],'complexity':'moderate'},
    {'id':'module:internal/provider/openai','type':'module','name':'openai','summary':'OpenAI-compatible /chat/completions provider package. Self-registers with provider.Register.','tags':['provider','openai','deepseek','minimax','llm'],'complexity':'complex'},
    {'id':'module:internal/skill','type':'module','name':'skill','summary':'Skill module defining built-in subagent/inline skills and tool discovery helpers.','tags':['skill','subagent','builtin'],'complexity':'moderate'},
    {'id':'module:internal/understand','type':'module','name':'understand','summary':'Understand module providing shared utilities for the Understand-Anything knowledge graph.','tags':['understand','knowledge-graph','prefix'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/openai.go:New','type':'function','name':'New','summary':'Factory building a provider.Provider from resolved Config. Validates URL/model, normalizes effort, detects vendor.','tags':['provider','factory','configuration'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/openai.go:Stream','type':'function','name':'Stream','summary':'Main streaming entry point. Marshals request, sends with retry, reads SSE stream asynchronously.','tags':['streaming','sse','retry','provider'],'complexity':'complex'},
    {'id':'function:internal/provider/openai/openai.go:readStream','type':'function','name':'readStream','summary':'Parses SSE stream into Chunk types: reasoning_content, content, tool-calls, usage, stall detection.','tags':['streaming','sse','parsing','tool-calls'],'complexity':'complex'},
    {'id':'function:internal/provider/openai/openai.go:buildRequest','type':'function','name':'buildRequest','summary':'Constructs chat request payload with tool-call pairing, DeepSeek reasoning_content round-trip, vision parts, thinking mode.','tags':['request','serialization','thinking','tool-calls'],'complexity':'complex'},
    {'id':'function:internal/provider/openai/openai.go:normaliseUsage','type':'function','name':'normaliseUsage','summary':'Normalises cache-hit/miss and reasoning token counts across DeepSeek, OpenAI, MiMo wire shapes.','tags':['usage','tokens','caching','normalization'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/openai.go:cleanCustomHeaders','type':'function','name':'cleanCustomHeaders','summary':'Filters custom HTTP headers removing reserved ones and empty entries.','tags':['headers','http','security'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/openai.go:newHTTPClient','type':'function','name':'newHTTPClient','summary':'Creates http.Client with proxy support, localhost exclusion, configurable timeouts.','tags':['http','proxy','timeout','network'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/openai.go:streamWithReconnect','type':'function','name':'streamWithReconnect','summary':'Drives readStream with mid-stream connection-drop replay up to 3 times.','tags':['streaming','reconnect','retry','resilience'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/openai.go:sendChunk','type':'function','name':'sendChunk','summary':'Non-blocking channel send with context cancellation awareness.','tags':['channel','concurrency','context'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/openai.go:normalizeReasoningProtocol','type':'function','name':'normalizeReasoningProtocol','summary':'Normalizes reasoning_protocol config to deepseek, openai, none, or empty.','tags':['configuration','normalization'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/openai.go:normalizeChatURL','type':'function','name':'normalizeChatURL','summary':'Normalizes chat URL using provided value or default /chat/completions suffix.','tags':['url','configuration'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/openai.go:reservedCustomHeader','type':'function','name':'reservedCustomHeader','summary':'Returns true if header is reserved (Authorization, Content-Type, Accept, Host).','tags':['headers','http','security'],'complexity':'simple'},
    {'id':'class:internal/provider/openai/openai.go:client','type':'class','name':'client','summary':'OpenAI-compatible provider implementation holding config and runtime state. Implements provider.Provider.','tags':['provider','openai','client'],'complexity':'moderate'},
    {'id':'class:internal/provider/openai/think.go:thinkSplitter','type':'class','name':'thinkSplitter','summary':'State machine peeling leading <think>...</think> blocks from MiniMax-M3 content stream.','tags':['thinking','streaming','minimax','state-machine'],'complexity':'moderate'},
    {'id':'function:internal/skill/builtins.go:builtinSkills','type':'function','name':'builtinSkills','summary':'Returns fresh slice of all built-in Skill definitions.','tags':['skill','builtin','definition'],'complexity':'simple'},
    {'id':'function:internal/skill/builtins.go:BuiltinNames','type':'function','name':'BuiltinNames','summary':'Returns names of all built-in skills.','tags':['skill','names'],'complexity':'simple'},
    {'id':'function:internal/skill/builtins.go:CodeGraphReadTools','type':'function','name':'CodeGraphReadTools','summary':'Returns read-only code graph MCP tool names from registry.','tags':['mcp','tools','discovery'],'complexity':'simple'},
    {'id':'function:internal/skill/builtins.go:WithCodeGraphTools','type':'function','name':'WithCodeGraphTools','summary':'Enriches code-reading subagent skills with code graph MCP tool names.','tags':['skill','enrichment','mcp','tools'],'complexity':'simple'},
    {'id':'function:internal/understand/prefix.go:Prefix','type':'function','name':'Prefix','summary':'Reads knowledge graph and returns cache-stable Markdown summary for system prompt.','tags':['prefix','knowledge-graph','system-prompt'],'complexity':'simple'},
    {'id':'function:internal/understand/prefix.go:buildPrefix','type':'function','name':'buildPrefix','summary':'Builds deterministic Markdown prefix from graph data. Capped at 2000 chars.','tags':['prefix','markdown','serialization'],'complexity':'moderate'},
    {'id':'concept:internal/provider/openai:EffortValidation','type':'concept','name':'EffortValidation','summary':'Vendor-specific reasoning effort validation for DeepSeek, MiniMax, OpenAI.','tags':['configuration','validation','thinking','effort'],'complexity':'moderate'},
    {'id':'concept:internal/provider/openai:WireShapeDetection','type':'concept','name':'WireShapeDetection','summary':'Auto-detection of wire protocol shape from base URL host. Overridable via reasoning_protocol config.','tags':['configuration','vendor-detection','thinking','protocol'],'complexity':'moderate'},
    {'id':'concept:internal/skill:CodeGraphToolDiscovery','type':'concept','name':'CodeGraphToolDiscovery','summary':'Discovers read-only code graph MCP tools and injects into built-in subagent skills.','tags':['mcp','tools','subagent','code-graph'],'complexity':'moderate'},
    {'id':'concept:internal/understand:SystemPromptPrefix','type':'concept','name':'SystemPromptPrefix','summary':'Injects cache-stable knowledge graph summary into system prompt.','tags':['system-prompt','caching','knowledge-graph','prefix'],'complexity':'simple'},
    {'id':'file:internal/provider/openai/host.go','type':'file','name':'host.go','summary':'Vendor host-matching utilities: IsDeepSeek, IsMiniMax, matchesVendorHost.','tags':['provider','openai','host-matching','vendor-detection'],'complexity':'simple'},
    {'id':'file:internal/provider/openai/think.go','type':'file','name':'think.go','summary':'MiniMax-M3 think tag splitter state machine.','tags':['provider','openai','thinking','minimax','state-machine'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/host.go:IsDeepSeek','type':'function','name':'IsDeepSeek','summary':'Reports whether baseURL points to DeepSeek API.','tags':['deepseek','vendor-detection'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/host.go:IsMiniMax','type':'function','name':'IsMiniMax','summary':'Reports whether baseURL points to MiniMax endpoint.','tags':['minimax','vendor-detection'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/host.go:matchesVendorHost','type':'function','name':'matchesVendorHost','summary':'Matches baseURL to canonical hostname or any subdomain of given apex domain.','tags':['url','host-matching','vendor-detection'],'complexity':'simple'},
    {'id':'function:internal/provider/openai/think.go:thinkSplitter.push','type':'function','name':'thinkSplitter.push','summary':'Pushes content delta through think-splitter state machine. Returns reasoning and text portions.','tags':['thinking','streaming','parsing','minimax'],'complexity':'moderate'},
    {'id':'function:internal/provider/openai/think.go:thinkSplitter.flush','type':'function','name':'thinkSplitter.flush','summary':'Emits buffered content at stream end.','tags':['thinking','streaming','buffering'],'complexity':'simple'},
]
batch4_edges = [
    {'source':'file:internal/provider/openai/openai.go','target':'file:internal/provider/openai/host.go','type':'imports','direction':'forward','weight':1.0},
    {'source':'file:internal/provider/openai/openai.go','target':'file:internal/provider/openai/think.go','type':'imports','direction':'forward','weight':1.0},
    {'source':'file:internal/provider/openai/openai.go','target':'file:internal/provider/openai/openai_test.go','type':'tested_by','direction':'bidirectional','weight':1.0},
    {'source':'file:internal/provider/openai/openai.go','target':'module:internal/provider/openai','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/provider/openai/host.go','target':'module:internal/provider/openai','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/provider/openai/think.go','target':'module:internal/provider/openai','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/provider/openai/openai_test.go','target':'module:internal/provider/openai','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/skill/builtins.go','target':'module:internal/skill','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/understand/prefix.go','target':'module:internal/understand','type':'contains','direction':'forward','weight':1.0},
    {'source':'module:internal/provider/openai','target':'concept:internal/provider/openai:EffortValidation','type':'contains','direction':'forward','weight':1.0},
    {'source':'module:internal/provider/openai','target':'concept:internal/provider/openai:WireShapeDetection','type':'contains','direction':'forward','weight':1.0},
    {'source':'module:internal/skill','target':'concept:internal/skill:CodeGraphToolDiscovery','type':'contains','direction':'forward','weight':1.0},
    {'source':'module:internal/understand','target':'concept:internal/understand:SystemPromptPrefix','type':'contains','direction':'forward','weight':1.0},
]

all_nodes = batch2_nodes + batch3_nodes + batch4_nodes
all_edges = batch2_edges + batch3_edges + batch4_edges

added_nodes = 0
for n in all_nodes:
    if n['id'] not in existing_ids:
        graph['nodes'].append(n)
        existing_ids.add(n['id'])
        added_nodes += 1

added_edges = 0
for e in all_edges:
    key = (e['source'], e['target'], e['type'])
    if key not in existing_edge_keys:
        graph['edges'].append(e)
        added_edges += 1
        existing_edge_keys.add(key)

print(f'Phase 2.5 (batches 2-4): Added {added_nodes} nodes, {added_edges} edges')
print(f'Graph now: {len(graph["nodes"])} nodes, {len(graph["edges"])} edges')

with open('.understand-anything/knowledge-graph.json', 'w', encoding='utf-8') as f:
    json.dump(graph, f, ensure_ascii=False, indent=1)
print('Saved')
