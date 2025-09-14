ocuroot("0.3.0")

def fn():
    value = "abc123"
    secret(value)
    print(value)
    return done()

task(fn=fn, name="fn")

value2 = "def456"
secret(value2)
print(value2)