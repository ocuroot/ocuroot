ocuroot("0.3.0")

def foo(i1, missing, i2="default"):
    return done()

task(
    foo, 
    name="foo",
    inputs={
        "i1": "a",
        "i2": "b",
    },
)