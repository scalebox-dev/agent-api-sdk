const SUPPORTED_VOLUME_IMAGE_EXTENSIONS = new Set([
  ".avif",
  ".bmp",
  ".gif",
  ".jpeg",
  ".jpg",
  ".png",
  ".svg",
  ".webp",
]);

export function normalizeVolumeAssetPath(src: string | null | undefined): string {
  const value = src?.trim() ?? "";
  if (!value) return "";
  if (isExternalAssetTarget(value)) return "";
  let path = value.split("#", 1)[0]?.split("?", 1)[0]?.trim() ?? "";
  path = path.replace(/^\/agent-volume\/+/i, "");
  path = path.replace(/^\/+/, "");
  path = path.replace(/\/+/g, "/");
  if (!path || path === "." || path.includes("..")) return "";
  return path;
}

export function isSupportedVolumeImagePath(src: string | null | undefined): boolean {
  const path = normalizeVolumeAssetPath(src).toLowerCase();
  if (!path) return false;
  const dot = path.lastIndexOf(".");
  return dot >= 0 && SUPPORTED_VOLUME_IMAGE_EXTENSIONS.has(path.slice(dot));
}

export function isSupportedVolumeImageContentType(contentType: string | null | undefined): boolean {
  return (contentType ?? "").split(";", 1)[0]?.trim().toLowerCase().startsWith("image/") ?? false;
}

function isExternalAssetTarget(src: string): boolean {
  return /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(src);
}
