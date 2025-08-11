ocuroot("0.3.0")

def foo(ctx):
    print(ctx)
    return next(
        bar, 
        inputs={
            "0startswithnumber": "a",
            "bad-input": "b",
        },
    )

def bar(ctx):
    print(ctx)
    return done()

phase(
    name="foo",
    work=[call(foo, name="foo")],
)