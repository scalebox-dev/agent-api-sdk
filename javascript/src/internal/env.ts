export function readEnv(name: string): string | undefined {
  const env = (globalThis as unknown as { process?: { env?: Record<string, string | undefined> } }).process?.env;
  return env?.[name];
}
