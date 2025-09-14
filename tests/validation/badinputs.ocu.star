ocuroot("0.3.0")

def foo():
    return done()

phase(
    name="foo",
    tasks=[task(foo, name="foo",
        inputs={
            "0startswithnumber": "a",
            "bad-input": "b",
        },
    )],
)