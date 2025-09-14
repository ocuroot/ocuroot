ocuroot("0.3.0")

def foo():
    return done(
        tags=[
            "invalid/tag",
        ]
    )

phase(
    name="foo",
    tasks=[task(foo, name="foo")],
)