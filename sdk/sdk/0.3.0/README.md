# Ocuroot SDK 0.3.0

This folder provides stubs implementing the 0.3.0 version of the Ocuroot SDK.

## Usage

All *.ocu.star files must declare the version
of the SDK they are using. This is done by calling the `ocuroot` function
with the version as the first argument.

For example:
ocuroot("0.3.0")

No additional load statements are needed to import the structs and functions
provided by the SDK. The contents of these files will be available to any
*.ocu.star file that calls `ocuroot("0.3.0")`.