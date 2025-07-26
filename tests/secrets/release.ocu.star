ocuroot("0.3.0")

def fn(ctx):
    value = "abc123"
    secret(value)
    print(value)
    return done()

phase(
    name="fn",
    work=[call(fn, name="fn")],
)

value2 = "def456"
secret(value2)
print(value2)