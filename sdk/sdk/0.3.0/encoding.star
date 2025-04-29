
json = struct(
    decode = lambda s: json.decode(s),
    encode = lambda o: json.encode(o),
)