import json

with open('.understand-anything/knowledge-graph.json', encoding='utf-8') as f:
    g = json.load(f)

changed = [
    'desktop/headroom_sidecar.go',
    'desktop/tab_profile_test.go',
    'desktop/tabs.go',
    'internal/agent/branch.go',
    'internal/control/controller.go',
]

# Collect ALL old nodes to remove (file + function/class/concept children)
remove_ids = set()
for n in g['nodes']:
    nid = n['id']
    for cf in changed:
        if nid == f'file:{cf}':
            remove_ids.add(nid)
        elif ':' in nid:
            rest = nid.split(':', 1)[1]
            if rest.startswith(cf + ':') or rest == cf:
                remove_ids.add(nid)

print(f'Nodes to remove: {len(remove_ids)}')
for nid in sorted(remove_ids):
    print(f'  REMOVE: {nid}')

g['nodes'] = [n for n in g['nodes'] if n['id'] not in remove_ids]
g['edges'] = [e for e in g['edges'] if e['source'] not in remove_ids and e['target'] not in remove_ids]
print(f'After cleanup: {len(g["nodes"])} nodes, {len(g["edges"])} edges')

# New nodes from sub-agent analysis
new_nodes = [
    # === headroom_sidecar.go (re-analyzed) ===
    {'id':'file:desktop/headroom_sidecar.go','type':'file','name':'headroom_sidecar.go','summary':'Headroom sidecar management for the Headroom context-compression proxy: lifecycle start/stop/health-check/stats/status, Windows launcher patch, and auto-start logic for app launch.','tags':['go','desktop','headroom','proxy','sidecar'],'complexity':'complex'},
    {'id':'class:desktop/headroom_sidecar.go:headroomSidecar','type':'class','name':'headroomSidecar','summary':'Wraps the headroom proxy subprocess with lifecycle management, health checks, stats fetching, and auto-start logic.','tags':['headroom','proxy','sidecar','lifecycle'],'complexity':'moderate'},
    {'id':'class:desktop/headroom_sidecar.go:HeadroomStatusView','type':'class','name':'HeadroomStatusView','summary':'Frontend-facing status view of headroom proxy with running state, stats, and configuration flags.','tags':['headroom','frontend','view'],'complexity':'moderate'},
    {'id':'class:desktop/headroom_sidecar.go:HeadroomConfigView','type':'class','name':'HeadroomConfigView','summary':'Carries headroom proxy settings from frontend including preset, mode, code-aware, kompress, GPU backend.','tags':['headroom','config','frontend'],'complexity':'simple'},
    {'id':'class:desktop/headroom_sidecar.go:proxyStats','type':'class','name':'proxyStats','summary':'Subset of /stats response fields: requests, tokens saved, savings percentage, lifetime stats.','tags':['headroom','stats'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:newHeadroomSidecar','type':'function','name':'newHeadroomSidecar','summary':'Constructor for headroomSidecar, initializes stopped channel.','tags':['headroom','constructor'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.installed','type':'function','name':'headroomSidecar.installed','summary':'Cached probe whether headroom CLI is available, runs headroom --version once.','tags':['headroom','cached'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.version','type':'function','name':'headroomSidecar.version','summary':'Cached headroom version string from CLI probe.','tags':['headroom','cached'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.start','type':'function','name':'headroomSidecar.start','summary':'Launches headroom proxy subprocess with config-driven env vars for compression engine, GPU backend, code-aware mode.','tags':['headroom','start','proxy'],'complexity':'complex'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.stop','type':'function','name':'headroomSidecar.stop','summary':'Terminates headroom proxy and its child process tree, uses taskkill on Windows.','tags':['headroom','stop','cleanup'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.status','type':'function','name':'headroomSidecar.status','summary':'Returns HeadroomStatusView with health check, installed/version info, and live stats.','tags':['headroom','status'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.healthCheck','type':'function','name':'headroomSidecar.healthCheck','summary':'Pings /livez endpoint with 3s timeout to check proxy reachability.','tags':['headroom','healthcheck'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.fetchStats','type':'function','name':'headroomSidecar.fetchStats','summary':'Fetches session and lifetime stats from /stats endpoint, decodes compression metrics.','tags':['headroom','stats'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.waitForReady','type':'function','name':'headroomSidecar.waitForReady','summary':'Polls health check every 200ms until proxy is ready or 10s timeout.','tags':['headroom','startup'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:writeWindowsLauncher','type':'function','name':'writeWindowsLauncher','summary':'Writes Python launcher script for headroom proxy on Windows to patch uvicorn event-loop policy.','tags':['headroom','windows','python'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:App.autoStartHeadroom','type':'function','name':'App.autoStartHeadroom','summary':'Auto-starts headroom proxy on app launch if enabled in config and any provider uses it.','tags':['headroom','auto-start','app'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:patchHeadroomServer','type':'function','name':'patchHeadroomServer','summary':'Patches headroom server.py for Python 3.12+ uvicorn LOOP_SETUPS compatibility.','tags':['headroom','patch','compatibility'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:HeadroomHasUpstream','type':'function','name':'HeadroomHasUpstream','summary':'Returns true if any enabled provider has headroom enabled.','tags':['headroom','config'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:upstreamURLForConfig','type':'function','name':'upstreamURLForConfig','summary':'Returns base URL of first headroom-enabled provider, appending /v1 for OpenAI-compatible APIs.','tags':['headroom','config'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:HeadroomStatus','type':'function','name':'HeadroomStatus','summary':'Wails binding returning headroom status view.','tags':['headroom','wails','binding'],'complexity':'simple'},
    {'id':'function:desktop/headroom_sidecar.go:StartHeadroom','type':'function','name':'StartHeadroom','summary':'Wails binding to start headroom proxy manually from frontend.','tags':['headroom','wails','binding'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.emitHeadroomStats','type':'function','name':'App.emitHeadroomStats','summary':'Fetches headroom stats and emits to frontend; handles auto-restart on unhealthy proxy and rebuilds controller on first ready.','tags':['headroom','frontend','stats'],'complexity':'complex'},
    {'id':'function:desktop/headroom_sidecar.go:headroomSidecar.pollHeadroomStats','type':'function','name':'App.pollHeadroomStats','summary':'Periodically fetches headroom proxy stats and emits frontend events every 5 seconds.','tags':['headroom','polling','frontend'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:App.SaveHeadroomConfig','type':'function','name':'App.SaveHeadroomConfig','summary':'Saves headroom proxy settings (preset, code-aware, kompress) and restarts proxy if runtime-affecting fields changed.','tags':['headroom','config','save'],'complexity':'moderate'},
    {'id':'function:desktop/headroom_sidecar.go:App.StopHeadroom','type':'function','name':'App.StopHeadroom','summary':'Wails binding to stop headroom proxy.','tags':['headroom','wails','binding'],'complexity':'simple'},

    # === tab_profile_test.go (NEW) ===
    {'id':'file:desktop/tab_profile_test.go','type':'file','name':'tab_profile_test.go','summary':'New test file for tab profile persistence and restoration, including effort persistence through BranchMeta.','tags':['go','test','tab','profile','effort'],'complexity':'moderate'},
    {'id':'function:desktop/tab_profile_test.go:TestEffortPersistedAndRestoredViaBranchMeta','type':'function','name':'TestEffortPersistedAndRestoredViaBranchMeta','summary':'Verifies effort (max) is saved to BranchMeta via saveTabSessionMeta and restored correctly via tabSessionProfile.','tags':['test','effort','branch-meta'],'complexity':'moderate'},
    {'id':'function:desktop/tab_profile_test.go:TestEffortNotOverwrittenWhenEmptyInMeta','type':'function','name':'TestEffortNotOverwrittenWhenEmptyInMeta','summary':'Verifies that when BranchMeta.Effort is empty (legacy session), a pre-existing tab effort is not overwritten during restore.','tags':['test','effort','legacy','backward-compat'],'complexity':'moderate'},

    # === tabs.go (re-analyzed) ===
    {'id':'file:desktop/tabs.go','type':'file','name':'tabs.go','summary':'Desktop tab management: workspace tab lifecycle, session path resolution, event routing, topic indexing, and project file management.','tags':['go','desktop','tabs','workspace','session'],'complexity':'complex'},
    {'id':'class:desktop/tabs.go:WorkspaceTab','type':'class','name':'WorkspaceTab','summary':'One open conversation tab owning independent controller, session, tool registry, permissions scoped to workspace root.','tags':['tab','workspace','session'],'complexity':'complex'},
    {'id':'function:desktop/tabs.go:saveTabSessionMeta','type':'function','name':'saveTabSessionMeta','summary':'Saves workspace tab session metadata to BranchMeta sidecar, including effort, mode, goal, and tool approval mode.','tags':['tab','session','meta','persistence'],'complexity':'moderate'},
    {'id':'function:desktop/tabs.go:tabSessionProfileFromMeta','type':'function','name':'tabSessionProfileFromMeta','summary':'Converts BranchMeta into tabSessionProfile, reading effort, mode, tool approval mode, and goal.','tags':['tab','profile','metadata'],'complexity':'moderate'},
    {'id':'function:desktop/tabs.go:applyTabSessionProfile','type':'function','name':'applyTabSessionProfile','summary':'Applies a tabSessionProfile to a workspace tab, setting effort, mode, tool approval mode, goal, and model.','tags':['tab','profile','restore'],'complexity':'moderate'},
    {'id':'class:desktop/tabs.go:tabSessionProfile','type':'class','name':'tabSessionProfile','summary':'Deserialized session profile from BranchMeta with model, effort, mode, token mode, tool approval mode, goal.','tags':['tab','session','profile'],'complexity':'simple'},

    # === branch.go (re-analyzed) ===
    {'id':'file:internal/agent/branch.go','type':'file','name':'branch.go','summary':'Branch metadata management: sidecar metadata for session files enabling conversation tree navigation, listing, and session info.','tags':['go','agent','branch','meta','session','sidecar'],'complexity':'moderate'},
    {'id':'class:internal/agent/branch.go:BranchMeta','type':'class','name':'BranchMeta','summary':'Sidecar record next to .jsonl session files storing model, effort, mode, goal, topic info, turns, preview, and schema version.','tags':['branch','meta','session'],'complexity':'moderate'},
    {'id':'class:internal/agent/branch.go:InFlightTurnMeta','type':'class','name':'InFlightTurnMeta','summary':'Records message-log boundary for an in-progress turn to handle mid-exit recovery.','tags':['branch','turn','recovery'],'complexity':'simple'},
    {'id':'function:internal/agent/branch.go:BranchID','type':'function','name':'BranchID','summary':'Derives branch ID from session file path using base filename without extension.','tags':['branch','id'],'complexity':'simple'},
    {'id':'function:internal/agent/branch.go:LoadBranchMeta','type':'function','name':'LoadBranchMeta','summary':'Reads and deserializes BranchMeta from .jsonl.meta sidecar file.','tags':['branch','load','meta'],'complexity':'simple'},
    {'id':'function:internal/agent/branch.go:SaveBranchMeta','type':'function','name':'SaveBranchMeta','summary':'Saves BranchMeta to sidecar file with atomic write via temp file + Rename.','tags':['branch','save','meta'],'complexity':'moderate'},
    {'id':'function:internal/agent/branch.go:UpdateSessionMeta','type':'function','name':'UpdateSessionMeta','summary':'Refreshes listing-only sidecar fields (model, preview, turn count) for sidebar/pickers, stamps SchemaVersion.','tags':['branch','update','listing'],'complexity':'moderate'},

    # === controller.go (re-analyzed — formatting only, maintain existing nodes) ===
    {'id':'file:internal/control/controller.go','type':'file','name':'controller.go','summary':'Transport-agnostic session controller: owns agent run loop, session lifecycle, command dispatch, event streaming.','tags':['go','controller','session','agent','run-loop'],'complexity':'complex'},
    {'id':'class:internal/control/controller.go:Controller','type':'class','name':'Controller','summary':'Drives one chat session with agent executor, guardian, memory, MCP plugins, goals, checkpoints, approval, and auto-research.','tags':['controller','session','agent'],'complexity':'complex'},
    {'id':'class:internal/control/controller.go:Options','type':'class','name':'Options','summary':'Carries all pre-built pieces for Controller construction: runner, executor, guardian, sink, policy, MCP host, skills, memory.','tags':['controller','options','config'],'complexity':'complex'},
    {'id':'function:internal/control/controller.go:New','type':'function','name':'New','summary':'Constructs a Controller with given options, replacing nil sink with event.Discard.','tags':['controller','constructor'],'complexity':'moderate'},
    {'id':'function:internal/control/controller.go:Controller.SetGoalWithResearchMode','type':'function','name':'SetGoalWithResearchMode','summary':'Sets a goal with optional auto-research mode; creates or resumes auto-research task if applicable.','tags':['goal','autoresearch','controller'],'complexity':'moderate'},
    {'id':'function:internal/control/controller.go:Controller.ensureAutoResearchTask','type':'function','name':'ensureAutoResearchTask','summary':'Creates or resumes auto-research task for a goal, returning task ID and optional block reason.','tags':['autoresearch','task','goal'],'complexity':'moderate'},
    {'id':'function:internal/control/controller.go:Controller.AutoResearchSummary','type':'function','name':'AutoResearchSummary','summary':'Returns summary of current auto-research task, including task ID, status, and iteration.','tags':['autoresearch','summary'],'complexity':'simple'},
    {'id':'function:internal/control/controller.go:Controller.AutoResearchList','type':'function','name':'AutoResearchList','summary':'Lists all auto-research task summaries for the session.','tags':['autoresearch','list'],'complexity':'simple'},
    {'id':'function:internal/control/controller.go:Controller.AutoResearchFindings','type':'function','name':'AutoResearchFindings','summary':'Returns findings for current auto-research task, capped at given limit.','tags':['autoresearch','findings'],'complexity':'simple'},
    {'id':'function:internal/control/controller.go:Controller.RecordAutoResearchEvidence','type':'function','name':'RecordAutoResearchEvidence','summary':'Records evidence for current auto-research task under a criterion ID.','tags':['autoresearch','evidence'],'complexity':'simple'},

    # === concept ===
    {'id':'concept:effort-persistence-through-branchmeta','type':'concept','name':'Effort Persistence Through BranchMeta','summary':'Tab-local effort setting is persisted to BranchMeta sidecar during saveTabSessionMeta, restored via tabSessionProfileFromMeta/applyTabSessionProfile. Empty effort in legacy sessions does not overwrite existing tab effort.','tags':['effort','persistence','branch-meta'],'complexity':'moderate'},
]

new_edges = [
    # tabs -> agent dependencies
    {'source':'file:desktop/tabs.go','target':'file:internal/agent/branch.go','type':'depends_on','direction':'forward','weight':0.8},
    {'source':'function:desktop/tabs.go:saveTabSessionMeta','target':'class:internal/agent/branch.go:BranchMeta','type':'depends_on','direction':'forward','weight':0.9},
    {'source':'function:desktop/tabs.go:tabSessionProfileFromMeta','target':'class:internal/agent/branch.go:BranchMeta','type':'depends_on','direction':'forward','weight':0.9},
    # tab_profile_test testing relationships
    {'source':'function:desktop/tab_profile_test.go:TestEffortPersistedAndRestoredViaBranchMeta','target':'function:desktop/tabs.go:saveTabSessionMeta','type':'tested_by','direction':'backward','weight':0.8},
    {'source':'function:desktop/tab_profile_test.go:TestEffortPersistedAndRestoredViaBranchMeta','target':'function:desktop/tabs.go:applyTabSessionProfile','type':'tested_by','direction':'backward','weight':0.8},
    {'source':'function:desktop/tab_profile_test.go:TestEffortPersistedAndRestoredViaBranchMeta','target':'function:internal/agent/branch.go:LoadBranchMeta','type':'tested_by','direction':'backward','weight':0.7},
    {'source':'function:desktop/tab_profile_test.go:TestEffortNotOverwrittenWhenEmptyInMeta','target':'class:internal/agent/branch.go:BranchMeta','type':'tested_by','direction':'backward','weight':0.6},
    # concept edges
    {'source':'concept:effort-persistence-through-branchmeta','target':'class:internal/agent/branch.go:BranchMeta','type':'related','direction':'bidirectional','weight':0.9},
    {'source':'concept:effort-persistence-through-branchmeta','target':'function:desktop/tabs.go:saveTabSessionMeta','type':'related','direction':'bidirectional','weight':0.9},
    {'source':'concept:effort-persistence-through-branchmeta','target':'file:desktop/tab_profile_test.go','type':'tested_by','direction':'backward','weight':0.7},
    # contain edges
    {'source':'file:desktop/headroom_sidecar.go','target':'class:desktop/headroom_sidecar.go:headroomSidecar','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:desktop/headroom_sidecar.go','target':'class:desktop/headroom_sidecar.go:HeadroomStatusView','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:desktop/headroom_sidecar.go','target':'class:desktop/headroom_sidecar.go:HeadroomConfigView','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:desktop/headroom_sidecar.go','target':'class:desktop/headroom_sidecar.go:proxyStats','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:desktop/tabs.go','target':'class:desktop/tabs.go:WorkspaceTab','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:desktop/tabs.go','target':'class:desktop/tabs.go:tabSessionProfile','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/agent/branch.go','target':'class:internal/agent/branch.go:BranchMeta','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/agent/branch.go','target':'class:internal/agent/branch.go:InFlightTurnMeta','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/control/controller.go','target':'class:internal/control/controller.go:Controller','type':'contains','direction':'forward','weight':1.0},
    {'source':'file:internal/control/controller.go','target':'class:internal/control/controller.go:Options','type':'contains','direction':'forward','weight':1.0},
]

existing_ids = {n['id'] for n in g['nodes']}
added_nodes = 0
for n in new_nodes:
    if n['id'] not in existing_ids:
        g['nodes'].append(n)
        existing_ids.add(n['id'])
        added_nodes += 1

existing_ek = {(e['source'],e['target'],e['type']) for e in g['edges']}
added_edges = 0
for e in new_edges:
    key = (e['source'],e['target'],e['type'])
    if key not in existing_ek:
        g['edges'].append(e)
        added_edges += 1
        existing_ek.add(key)

print(f'Added: {added_nodes} nodes, {added_edges} edges')
print(f'Total: {len(g["nodes"])} nodes, {len(g["edges"])} edges')

with open('.understand-anything/knowledge-graph.json', 'w', encoding='utf-8') as f:
    json.dump(g, f, ensure_ascii=False, indent=1)
print('Saved')
