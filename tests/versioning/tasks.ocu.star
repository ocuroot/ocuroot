ocuroot("0.3.0")

def build(ctx):
    print("Building")
    return done()

def do_release(version):
    print("Releasing {}".format(version))