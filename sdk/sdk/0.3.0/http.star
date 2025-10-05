def normalizeHeaders(headers={}):
    normalized = {}
    for k, v in headers.items():
        if type(v) != "list":
            v = [v]
        normalized[k] = v
    return normalized

def _post(url, headers={}, body=""):
    resp = backend.http.req(json.encode({
        "method": "POST",
        "url": url,
        "headers": normalizeHeaders(headers),
        "body": body,
    }))
    return _response(resp)

def _get(url, headers={}):
    resp = backend.http.req(json.encode({
        "method": "GET",
        "url": url,
        "headers": normalizeHeaders(headers),
    }))
    return _response(resp)

def _head(url, headers={}):
    resp = backend.http.req(json.encode({
        "method": "HEAD",
        "url": url,
        "headers": normalizeHeaders(headers),
    }))
    return _response(resp)

def _put(url, headers={}, body=""):
    resp = backend.http.req(json.encode({
        "method": "PUT",
        "url": url,
        "headers": normalizeHeaders(headers),
        "body": body,
    }))
    return _response(resp)

def _patch(url, headers={}, body=""):
    resp = backend.http.req(json.encode({
        "method": "PATCH",
        "url": url,
        "headers": normalizeHeaders(headers),
        "body": body,
    }))
    return _response(resp)

def _delete(url, headers={}):
    resp = backend.http.req(json.encode({
        "method": "DELETE",
        "url": url,
        "headers": normalizeHeaders(headers),
    }))
    return _response(resp)

def _response(resp_json):
    resp = json.decode(resp_json)
    return struct(
        status_code = resp.get("status_code"),
        status_text = resp.get("status_text"),
        headers = resp.get("headers"),
        body = resp.get("body"),
    )

http = struct(
    get = _get,
    post = _post,
    head = _head,
    put = _put,
    patch = _patch,
    delete = _delete,
)