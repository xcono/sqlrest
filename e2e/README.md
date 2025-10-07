# E2E Testing Package

This package contains end-to-end tests that compare sqlrest behavior with PostgREST to ensure compatibility.

## Test Structure

### Core Tests (`e2e_test.go`)
- **Basic functionality tests**: SELECT, filtering, ordering, pagination
- **Compatibility tests**: Ensure sqlrest responses match PostgREST responses
- **Build tag**: `//go:build e2e`

### Incompatibility Tests (`incompat_test.go`)
- **Edge case documentation**: Known differences between PostgREST and sqlrest
- **Platform differences**: Database-specific behavior (collation, ordering, etc.)
- **Non-failing tests**: Document incompatibilities without causing test failures

## Running Tests

### Run all e2e tests
```bash
go test -tags e2e ./e2e
```

### Run specific test files
```bash
go test -tags e2e ./e2e -run TestE2EComparison
go test -tags e2e ./e2e -run TestIncompatibilities
```

### Run tests in parallel (faster)
```bash
go test -tags e2e -parallel 4 ./e2e
```

## Test Environment

### Prerequisites
- Docker daemon running (for Testcontainers)
- No manual setup required - containers are managed automatically

### Container Infrastructure
The tests use Testcontainers to automatically manage:
- **MySQL 8.4**: Database for sqlrest testing
- **PostgreSQL 17.5**: Database for PostgREST testing  
- **PostgREST v12.2.3**: REST API server for comparison testing
- **Docker Network**: Enables communication between PostgreSQL and PostgREST

### Database Setup
The tests use a simple dataset with the following tables:
- `artist` - Music artists
- `album` - Music albums
- `track` - Music tracks
- `genre` - Music genres
- `playlist_track` - Playlist-track relationships

### Test Data
- **Simple dataset**: 5 artists, 5 albums, 10 tracks, 5 genres
- **Lowercase naming**: All tables and columns use snake_case
- **Consistent data**: Same data seeded in both MySQL and PostgreSQL
- **Automatic seeding**: Migration files are automatically applied during container startup

## Test Categories

### 1. Core Compatibility Tests
- `select_all_artists` - Basic SELECT without filters
- `select_artist_columns` - Column selection
- `filter_eq` - Equality filtering
- `filter_gt` - Greater than filtering
- `filter_in` - IN clause filtering
- `order_asc` - Ascending ordering
- `order_desc` - Descending ordering
- `limit_offset` - Pagination
- `complex_query` - Combined filters and selections

### 2. Incompatibility Documentation
- `ordering_with_ties` - Different ordering when there are ties
- `limit_offset_without_order` - Non-deterministic ordering without ORDER BY
- `collation_differences` - Database collation differences
- `numeric_precision` - Different numeric precision handling
- `case_sensitivity` - Different case sensitivity behavior

## Extending Tests

### Adding New Test Cases

1. **Core functionality tests**: Add to `e2e_test.go`
   ```go
   t.Run("new_feature", func(t *testing.T) {
       // Test implementation
   })
   ```

2. **Incompatibility tests**: Add to `incompat_test.go`
   ```go
   t.Run("new_incompatibility", func(t *testing.T) {
       // Document the incompatibility
       t.Logf("Expected incompatibility: %v", err)
   })
   ```

### Test Data Management

- **Migration files**: `migrations/my/simple_mysql.sql` and `migrations/pg/simple_postgres.sql`
- **Seeding**: `dbseed/seed.go` handles database seeding
- **Cleanup**: Tests automatically clean up after themselves

### Comparison Logic

- **Response comparison**: `compare/compare.go` handles semantic comparison
- **Type normalization**: Converts database types to comparable formats
- **Ordering handling**: Smart sorting for order-independent comparison

## Known Incompatibilities

### 1. Ordering with Ties
**Issue**: When ORDER BY has ties, PostgreSQL and MySQL may return different orders
**Cause**: Different collation settings and tie-breaking behavior
**Solution**: Add secondary sort keys for deterministic ordering

### 2. LIMIT/OFFSET without ORDER BY
**Issue**: Without explicit ORDER BY, databases may return results in different orders
**Cause**: Non-deterministic default ordering
**Solution**: Always use ORDER BY with LIMIT/OFFSET

### 3. Numeric Precision
**Issue**: DECIMAL types may be returned with different precision
**Cause**: Database-specific numeric handling
**Solution**: Use explicit precision specifications

## Best Practices

### Test Writing
- Use descriptive test names
- Add comments explaining test purpose
- Use `t.Logf()` for debugging information
- Handle expected incompatibilities gracefully

### Data Management
- Keep test data simple and focused
- Use consistent naming conventions
- Clean up test data after tests
- Use transactions for test isolation

### Performance
- Use `t.Parallel()` for independent tests
- Minimize database operations
- Use build tags to separate test types
- Run tests in parallel when possible

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Run E2E Tests
  run: |
    # Ensure Docker daemon is running
    sudo systemctl start docker
    # Run tests (containers managed automatically)
    go test -tags e2e ./e2e
```

### Test Selection
```bash
# Run only core compatibility tests
go test -tags e2e ./e2e -run TestE2EComparison

# Run only incompatibility documentation
go test -tags e2e ./e2e -run TestIncompatibilities

# Run specific test case
go test -tags e2e ./e2e -run TestE2EComparison/select_all_artists
```

## Troubleshooting

### Common Issues

1. **Docker daemon not running**
   - Ensure Docker daemon is started: `sudo systemctl start docker`
   - Verify Docker is accessible: `docker ps`

2. **Container startup failures**
   - Check Docker has sufficient resources (memory, disk space)
   - Verify migration files exist and are readable
   - Check network connectivity for image downloads

3. **Ordering differences**
   - Check if test is in `incompat_test.go`
   - Verify if it's a known platform difference
   - Consider adding secondary sort keys

### Debugging

- Use `t.Logf()` to output debug information
- Check test logs for database queries
- Verify response data matches expectations
- Use `-v` flag for verbose test output

## Future Enhancements

### Planned Features
- [ ] Performance benchmarking tests
- [ ] Load testing with multiple concurrent requests
- [ ] Error handling compatibility tests
- [ ] Authentication and authorization tests
- [ ] Complex JOIN operation tests
- [ ] JSON aggregation tests

### Test Data Expansion
- [ ] Larger datasets for performance testing
- [ ] Edge case data (nulls, empty strings, special characters)
- [ ] Unicode and internationalization data
- [ ] Complex relationship data

### Infrastructure Improvements
- [x] Automated test environment setup (Testcontainers)
- [ ] Test data generation utilities
- [ ] Test result reporting and visualization
- [ ] Integration with monitoring tools