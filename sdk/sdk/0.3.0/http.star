def normalizeHeaders(headers={}):
    normalized = {}
    for k, v in headers.items():
        if type(v) != "list":
            v = [v]
        normalized[k] = v
    return normalized

def _post(url, headers={}, body=""):
    resp = backend.http.post(json.encode({
        "url": url,
        "headers": normalizeHeaders(headers),
        "body": body,
    }))
    return json.decode(resp)

def _get(url, headers={}):
    resp = backend.http.get(json.encode({
        "url": url,
        "headers": normalizeHeaders(headers),
    }))
    return json.decode(resp)

http = struct(
    get = _get,
    post = _post,
)