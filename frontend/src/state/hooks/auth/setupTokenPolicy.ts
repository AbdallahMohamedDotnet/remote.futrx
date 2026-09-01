// The first-boot setup token reaches the browser in the URL fragment rather
// than the query string. Browsers never send a fragment to the server, so the
// token cannot land in a reverse-proxy access log the way "?token=" would -
// which is the whole reason it is not simply a query parameter.
class SetupTokenPolicy {
  // read pulls the token out of a location hash. It tolerates the hash being
  // absent, empty, or carrying unrelated parameters.
  read(rawHash: string): string {
    const hash = rawHash.startsWith("#") ? rawHash.slice(1) : rawHash;
    if (!hash) return "";
    return new URLSearchParams(hash).get("token")?.trim() ?? "";
  }

  // strippedUrl is the address to rewrite to once the token has been read into
  // memory, so it stops sitting in the address bar and the history entry.
  strippedUrl(pathname: string, search: string): string {
    return `${pathname}${search}`;
  }
}

export const setupTokenPolicy = new SetupTokenPolicy();
