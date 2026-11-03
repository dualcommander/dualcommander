// VS Code strips NODE_OPTIONS from extension hosts but inherits server execArgv.
if (process.argv[1]?.endsWith('/out/server-main.js')) {
  process.execArgv.push('--max-old-space-size=4096');
}