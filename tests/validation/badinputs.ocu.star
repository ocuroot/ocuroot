ocuroot("0.3.0")

def foo(ctx):
    print(ctx)
    return done()

phase(
    name="foo",
    work=[call(foo, name="foo",
        inputs={
            "0startswithnumber": "a",
            "bad-input": "b",
        },
    )],
)