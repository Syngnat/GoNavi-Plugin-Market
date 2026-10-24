// Manual protocol probe: initialize then invoke, print frames and stderr.
import { spawn } from 'node:child_process';

const exe = process.argv[2] ?? 'dist/package/bin/plugin-windows-amd64.exe';
const command = process.argv[3] ?? 'connections.list';
const args = process.argv[4] ?? 'null';

const child = spawn(exe, [], { stdio: ['pipe', 'pipe', 'pipe'] });
let buffer = '';
let initialized = false;
child.stdout.on('data', (chunk) => {
  buffer += chunk.toString('utf8');
  let idx;
  while ((idx = buffer.indexOf('\n')) >= 0) {
    const line = buffer.slice(0, idx);
    buffer = buffer.slice(idx + 1);
    if (!line.trim()) continue;
    console.log('STDOUT:', line);
    // Fire the invoke only after the initialize reply so there is no race.
    if (!initialized && line.includes('"id":1')) {
      initialized = true;
      if (args === 'OMIT') {
        send({ jsonrpc: '2.0', id: 2, method: 'plugin.invoke', params: { commandId: command } });
      } else {
        send({ jsonrpc: '2.0', id: 2, method: 'plugin.invoke', params: { commandId: command, arguments: JSON.parse(args) } });
      }
    }
  }
});
child.stderr.on('data', (chunk) => console.log('STDERR:', chunk.toString('utf8')));
child.on('exit', (code) => console.log('EXIT:', code));

const send = (frame) => child.stdin.write(JSON.stringify(frame) + '\n');
send({ jsonrpc: '2.0', id: 1, method: 'plugin.initialize', params: { apiVersion: 1, pluginId: 'com.gonavi.nginx-inspector', version: '1.0.0', generation: 1, config: {}, hostVersion: '1.1.5', locale: 'zh-CN', permissions: ['ai.tools', 'ai.skills', 'host.log'] } });
setTimeout(() => { send({ jsonrpc: '2.0', id: 3, method: 'plugin.shutdown', params: {} }); }, 2500);
setTimeout(() => child.kill(), 4000);
