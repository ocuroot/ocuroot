ocuroot("0.3.0")

def foo():
    return done(
        tags=[
            "r123",
        ]
    )

phase(
    name="foo",
    tasks=[task(foo, name="foo")],
)