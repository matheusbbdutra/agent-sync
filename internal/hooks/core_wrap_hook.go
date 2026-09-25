package hooks

import "path/filepath"

// wrapHookCommand envelopa um script de hook via hooks/wrap-hook.sh. O wrapper
// e' quem captura erros de execucao via observe-error.sh para metricas.
func wrapHookCommand(baseDir, stage, hookName, scriptPath string) string {
	return wrapHookCommandWithCLI(baseDir, stage, hookName, scriptPath, "")
}

// wrapHookCommandWithCLI envelopa um script de hook passando a CLI explicitamente (--cli=<cli>).
func wrapHookCommandWithCLI(baseDir, stage, hookName, scriptPath, cli string) string {
	wrapper := filepath.Join(baseDir, "hooks", "wrap-hook.sh")
	if cli != "" {
		return wrapper + " --cli=" + cli + " " + stage + " " + hookName + " " + scriptPath
	}
	return wrapper + " " + stage + " " + hookName + " " + scriptPath
}
