// File names the server can extract (see http/archive.go).
const EXTRACTABLE = [
  ".zip",
  ".tar",
  ".tar.gz",
  ".tgz",
  ".tar.bz2",
  ".tbz2",
  ".tar.xz",
  ".txz",
  ".tar.zst",
  ".tzst",
  ".7z",
  ".rar",
];

export const isArchiveName = (name: string) => {
  const lower = name.toLowerCase();
  return EXTRACTABLE.some((ext) => lower.endsWith(ext));
};

// "photos.tar.gz" -> "photos": the folder an archive is extracted into.
export const archiveBaseName = (name: string) => {
  const lower = name.toLowerCase();
  const ext = EXTRACTABLE.filter((e) => lower.endsWith(e)).sort(
    (a, b) => b.length - a.length
  )[0];
  return ext ? name.slice(0, name.length - ext.length) : name;
};
