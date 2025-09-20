ocuroot("0.3.14")

def build():
    print("Building with SDK version 0.3.14")
    print("This demonstrates version aliasing: 0.3.14 -> 0.3.0")
    
    # Test basic SDK functionality
    res = shell("echo 'Version test'", mute=True)
    print("Shell command result:", res.stdout.strip())
    
    print("Host info - OS:", os(), "Arch:", arch())
    
    return done(
        outputs={
            "version_used": "0.3.14",
            "test_result": "success",
        },
    )

def deploy_test(version_used, test_result):
    print("Deploying with version:", version_used)
    print("Test result:", test_result)
    return done()