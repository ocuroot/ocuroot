# Demo file for testing REPL pretty-printing

def greet(name, greeting="Hello"):
    """Greets a person with a customizable greeting"""
    return greeting + ", " + name + "!"

def calculate(x, y, *args, **kwargs):
    """Performs calculations with variable arguments"""
    result = x + y
    for arg in args:
        result += arg
    return result

# Some test data structures
simple_list = [1, 2, 3, 4, 5]
nested_list = [1, [2, 3], [4, [5, 6]]]
simple_dict = {"name": "Alice", "age": 30, "city": "NYC"}
nested_dict = {
    "user": {
        "name": "Bob",
        "details": {
            "age": 25,
            "hobbies": ["reading", "coding"]
        }
    }
}
