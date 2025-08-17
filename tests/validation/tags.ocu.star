ocuroot("0.3.0")

def foo(ctx):
    print(ctx)
    return done(
        tags=[
            "invalid/tag",
        ]
    )

phase(
    name="foo",
    work=[call(foo, name="foo")],
)