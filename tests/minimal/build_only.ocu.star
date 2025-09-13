ocuroot("0.3.0")

def build():
    print("build")
    return done()

task(build, "build")