# Testing Guide

This document provides comprehensive information about testing in the album2buy project.

## Overview

The album2buy project includes a robust testing suite with **73.9% code coverage** across all components. The testing strategy focuses on reliability, maintainability, and confidence in the application's functionality.

## Test Structure

### Unit Tests (`main_test.go`)
Tests individual components in isolation:

- **HTTPClient**: Retry logic, context cancellation, TLS configuration
- **LastFMClient**: API calls, JSON parsing, error handling
- **SubsonicClient**: Search functionality, album detection, authentication
- **ProgressIndicator**: Spinner and progress bar functionality
- **Utility Functions**: String cleaning, URL filtering, configuration loading
- **Output Functions**: Recommendation formatting and display

### Integration Tests (`integration_test.go`)
Tests complete workflows and component interactions:

- **End-to-End Workflow**: Last.fm → Subsonic → Recommendations
- **Ignored URLs**: File-based URL filtering functionality
- **Maximum Recommendations**: Limit enforcement testing
- **Error Scenarios**: Network failures, invalid responses

## Running Tests

### Basic Test Commands

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific test patterns
go test -run TestHTTPClient
go test -run TestLastFMClient
go test -run TestSubsonicClient
go test -run Integration
```

### Coverage Analysis

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Show coverage by function
go tool cover -func=coverage.out
```

### Performance Testing

```bash
# Run tests with benchmarking
go test -bench=. ./...

# Profile memory usage
go test -memprofile=mem.prof ./...

# Profile CPU usage
go test -cpuprofile=cpu.prof ./...
```

## Test Categories

### 1. HTTP Client Tests
- **Retry Logic**: Verifies automatic retry on failures
- **Context Handling**: Tests timeout and cancellation scenarios
- **TLS Configuration**: Validates secure connection settings
- **Error Handling**: Ensures proper error propagation

### 2. API Client Tests
- **Last.fm Integration**: Mock server responses, JSON parsing
- **Subsonic Integration**: Authentication, search functionality
- **Error Scenarios**: Invalid JSON, network failures, API errors

### 3. Utility Function Tests
- **String Cleaning**: Album/artist name normalization
- **URL Filtering**: Ignore list functionality
- **Configuration**: Environment variable handling
- **File Operations**: Ignore file parsing, error handling

### 4. Integration Tests
- **Complete Workflows**: Full application flow testing
- **Mock Services**: HTTP test servers for external APIs
- **Data Flow**: Verification of data transformation pipeline

## Testing Best Practices

### 1. Mock External Dependencies
- Use `httptest.NewServer()` for API mocking
- Create realistic response data
- Test both success and failure scenarios

### 2. Environment Isolation
- Save and restore environment variables
- Use temporary files for file operations
- Clean up resources in `defer` statements

### 3. Comprehensive Coverage
- Test all public functions and methods
- Include edge cases and error conditions
- Verify both positive and negative scenarios

### 4. Clear Test Names
- Use descriptive test function names
- Structure: `TestComponentFunctionality`
- Example: `TestHTTPClientDoWithRetrySuccess`

## Mock Strategy

### HTTP Server Mocking
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Mock response logic
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("mock response"))
}))
defer server.Close()
```

### Environment Variable Mocking
```go
originalValue := os.Getenv("TEST_VAR")
os.Setenv("TEST_VAR", "test-value")
defer func() {
    if originalValue == "" {
        os.Unsetenv("TEST_VAR")
    } else {
        os.Setenv("TEST_VAR", originalValue)
    }
}()
```

### Temporary File Creation
```go
tmpFile, err := os.CreateTemp("", "test-file")
if err != nil {
    t.Fatal(err)
}
defer os.Remove(tmpFile.Name())
```

## Continuous Integration

### Test Automation
- All tests run automatically on code changes
- Coverage reports generated for each build
- Failed tests block deployments

### Quality Gates
- Minimum 70% code coverage required
- All tests must pass before merge
- Static analysis with `go vet` and `gofmt`

## Troubleshooting

### Common Issues

1. **Test Timeout**: Increase context timeout for slow operations
2. **Port Conflicts**: Use `httptest.NewServer()` for dynamic ports
3. **Environment Pollution**: Always restore original environment state
4. **Resource Leaks**: Use `defer` for cleanup operations

### Debugging Tests

```bash
# Run single test with verbose output
go test -v -run TestSpecificFunction

# Enable race detection
go test -race ./...

# Show test execution time
go test -v -count=1 ./...
```

## Contributing

When adding new functionality:

1. **Write Tests First**: Follow TDD principles when possible
2. **Maintain Coverage**: Aim for >70% coverage on new code
3. **Test Edge Cases**: Include error conditions and boundary cases
4. **Update Documentation**: Add test descriptions to this guide

### Test Checklist

- [ ] Unit tests for new functions
- [ ] Integration tests for new workflows
- [ ] Error condition testing
- [ ] Mock external dependencies
- [ ] Clean up resources
- [ ] Update coverage expectations
- [ ] Verify CI pipeline passes