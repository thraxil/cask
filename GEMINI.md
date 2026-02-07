# Gemini CLI Interaction Guidelines

This document outlines guidelines for effective interaction with the Gemini CLI agent in the `cask` project.

## Project Overview

`cask` is a distributed key-value store written in Go.

## Project-Specific Commands

You can use the following `make` commands to build and test the project:

*   `make cask`: Build the `cask` binary.
*   `make test`: Run the test suite.
*   `make cluster`: Run a local cluster for testing.

## Example Interactions

*   **To read a file:** `read_file(file_path='cask.go')`
*   **To search for a function:** `search_file_content(pattern='func handleSet', include='*.go')`
*   **To run the tests:** `run_shell_command(command='make test')`

## General Best Practices
*   Be clear and concise in your requests.
*   Provide as much context as necessary, especially for code modifications.
*   If asking for code changes, specify the file paths and the exact code snippets to modify.
*   Review the agent's proposed changes or actions carefully before confirming.

## Troubleshooting and Feedback

*   If you encounter unexpected behavior, please provide detailed steps to reproduce.
*   For any feedback or to report a bug, use the `/bug` command.