class ValidationError(MemoryError):
    """Raised when input validation fails.

    This exception is raised when request parameters, memory content,
    or configuration values fail validation checks.

    Common scenarios:
        - Invalid user_id format
        - Missing required fields
        - content too long or too short
        - Invalid metadata format
        - Malformed filters

    Example:
        raise ValidatinError(
            message="Invalid user_id format",
            error_code="VAL_001",
            details={"field": "user_id", "value": "123", "expected": "string"},
            suggestion="User ID must be a non-empty string"
        )
    """
    pass
    