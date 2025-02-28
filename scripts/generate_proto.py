#!/usr/bin/env python3
"""
Script to generate Python gRPC code from proto files.
"""
import os
import subprocess
import sys

def generate_proto_files():
    # Get the project root directory
    current_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(current_dir)
    proto_dir = os.path.join(project_root, 'proto')

    # Ensure the proto directory exists
    if not os.path.isdir(proto_dir):
        print(f"Error: Proto directory not found at {proto_dir}")
        sys.exit(1)

    # Ensure the proto file exists
    proto_file = os.path.join(proto_dir, 'trading.proto')
    if not os.path.isfile(proto_file):
        print(f"Error: Proto file not found at {proto_file}")
        sys.exit(1)

    # Create __init__.py file if it doesn't exist
    init_file = os.path.join(proto_dir, '__init__.py')
    if not os.path.isfile(init_file):
        with open(init_file, 'w') as f:
            f.write("# This file makes the proto directory importable as a Python package\n")

    # Set up the command to generate the protobuf files
    cmd = [
        sys.executable, "-m", "grpc_tools.protoc",
        f"-I{proto_dir}",
        f"--python_out={proto_dir}",
        f"--grpc_python_out={proto_dir}",
        proto_file
    ]

    # Run the command
    print(f"Generating protobuf files from {proto_file}...")
    try:
        subprocess.check_call(cmd)
        print("Successfully generated protobuf files.")
    except subprocess.CalledProcessError as e:
        print(f"Error generating protobuf files: {e}")
        sys.exit(1)

    # Verify the generated files exist
    pb2_file = os.path.join(proto_dir, 'trading_pb2.py')
    pb2_grpc_file = os.path.join(proto_dir, 'trading_pb2_grpc.py')

    if not os.path.isfile(pb2_file) or not os.path.isfile(pb2_grpc_file):
        print("Error: Generated files not found.")
        sys.exit(1)

    print(f"Generated files:")
    print(f"  - {pb2_file}")
    print(f"  - {pb2_grpc_file}")

if __name__ == "__main__":
    generate_proto_files()