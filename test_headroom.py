import json, urllib.request, os

lines = ['Searching for: HandleFunc|Handle|func Test', '']
for i in range(150):
    mod = i % 8
    file = f'src/pkg{mod}/handler{i//3}.go'
    lines.append(f'{file}:{i*15+7}:    func (s *Server) HandleRequest{i}(w http.ResponseWriter, r *http.Request) {{')
    lines.append(f'{file}:{i*15+8}:        ctx := r.Context()')
    lines.append(f'{file}:{i*15+9}:        id := mux.Vars(r)["id"]')
    lines.append(f'{file}:{i*15+10}:       result, err := s.service.Process{mod}(ctx, id)')
    lines.append(f'{file}:{i*15+11}:       if err != nil {{')
    lines.append(f'{file}:{i*15+12}:           http.Error(w, err.Error(), http.StatusInternalServerError)')

tool = '\n'.join(lines)

body = json.dumps({
    'model': 'deepseek-chat',
    'messages': [
        {'role': 'system', 'content': 'You are a programming assistant.'},
        {'role': 'user', 'content': 'Find all HTTP handlers in the codebase.'},
        {'role': 'assistant', 'content': None, 'tool_calls': [{'id': 'call_x', 'type': 'function', 'function': {'name': 'grep', 'arguments': '{"pattern": "HandleFunc|Handle"}'}}]},
        {'role': 'tool', 'tool_call_id': 'call_x', 'name': 'grep', 'content': tool},
    ],
    'max_tokens': 30,
    'stream': False,
    'temperature': 0,
}).encode('utf-8')

req = urllib.request.Request(
    'http://127.0.0.1:8787/v1/chat/completions',
    data=body,
    headers={
        'Content-Type': 'application/json',
        'Authorization': f'Bearer {os.environ.get("DEEPSEEK_API_KEY", "")}',
    }
)

try:
    resp = urllib.request.urlopen(req, timeout=60)
    result = json.loads(resp.read())
    usage = result.get('usage', {})
    print(f'Model: {result.get("model", "?")}')
    print(f'Finish: {result["choices"][0].get("finish_reason", "?")}')
    content = result['choices'][0].get('message', {}).get('content', '') or ''
    print(f'Response: {content[:80]}...')
    print(f'Prompt tokens: {usage.get("prompt_tokens", "?")}')
    print(f'Total tokens: {usage.get("total_tokens", "?")}')
except Exception as e:
    print(f'Error: {e}')
