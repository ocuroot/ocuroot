ocuroot("0.3.0")

def foo():
    return next(
        bar, 
        inputs={
            "0startswithnumber": "a",
            "bad-input": "b",
        },
    )

def bar():
    return done()

phase(
    name="foo",
    tasks=[task(foo, name="foo")],
)