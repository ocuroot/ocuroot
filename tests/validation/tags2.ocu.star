ocuroot("0.3.0")

def foo(ctx):
    print(ctx)
    return done(
        tags=[
            "r123",
        ]
    )

phase(
    name="foo",
    work=[call(foo, name="foo")],
)