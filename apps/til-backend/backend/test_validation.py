from validation import WHAT_MAX_LENGTH, WHERE_DETAIL_MAX_LENGTH, WHO_MAX_LENGTH, validate_submission_payload


def test_valid_payload_passes():
    assert validate_submission_payload(
        {
            "who": "Jane Doe, CSM",
            "where": "Customer",
            "whereDetail": "Acme Corp",
            "what": "Learned something.",
        }
    ) is None


def test_valid_payload_passes_for_internal_with_no_where_detail():
    assert validate_submission_payload(
        {"who": "Jane Doe, CSM", "where": "Internal", "what": "Learned something."}
    ) is None


def test_where_detail_required_for_customer():
    err = validate_submission_payload({"who": "Jane", "where": "Customer", "what": "x"})
    assert err is not None and "whereDetail" in err


def test_where_detail_required_for_partner():
    err = validate_submission_payload({"who": "Jane", "where": "Partner", "what": "x"})
    assert err is not None and "whereDetail" in err


def test_blank_where_detail_rejected_for_customer():
    err = validate_submission_payload(
        {"who": "Jane", "where": "Customer", "whereDetail": "   ", "what": "x"}
    )
    assert err is not None and "whereDetail" in err


def test_where_detail_too_long_rejected():
    err = validate_submission_payload(
        {
            "who": "Jane",
            "where": "Customer",
            "whereDetail": "a" * (WHERE_DETAIL_MAX_LENGTH + 1),
            "what": "x",
        }
    )
    assert err is not None and "whereDetail" in err


def test_missing_who_rejected():
    err = validate_submission_payload({"where": "Customer", "what": "x"})
    assert err is not None and "who" in err


def test_blank_who_rejected():
    err = validate_submission_payload({"who": "   ", "where": "Customer", "what": "x"})
    assert err is not None and "who" in err


def test_who_too_long_rejected():
    err = validate_submission_payload(
        {"who": "a" * (WHO_MAX_LENGTH + 1), "where": "Customer", "what": "x"}
    )
    assert err is not None and "who" in err


def test_invalid_where_rejected():
    err = validate_submission_payload({"who": "Jane", "where": "Supplier", "what": "x"})
    assert err is not None and "where" in err


def test_missing_what_rejected():
    err = validate_submission_payload({"who": "Jane", "where": "Internal"})
    assert err is not None and "what" in err


def test_what_too_long_rejected():
    err = validate_submission_payload(
        {"who": "Jane", "where": "Internal", "what": "a" * (WHAT_MAX_LENGTH + 1)}
    )
    assert err is not None and "what" in err


def test_what_at_exact_limit_accepted():
    assert validate_submission_payload(
        {"who": "Jane", "where": "Internal", "what": "a" * WHAT_MAX_LENGTH}
    ) is None


def test_quill_empty_markup_rejected_as_blank():
    # A rich-text editor with nothing typed still sends "<p><br></p>", not
    # "" -- the empty check has to read past the markup, same as the
    # frontend's own isEmptyTilHtml.
    err = validate_submission_payload({"who": "Jane", "where": "Internal", "what": "<p><br></p>"})
    assert err is not None and "what" in err


def test_what_length_counts_plain_text_not_markup():
    # Real content wrapped in tags must not count the tags themselves
    # towards the limit.
    html = f"<p><strong>{'a' * WHAT_MAX_LENGTH}</strong></p>"
    assert validate_submission_payload({"who": "Jane", "where": "Internal", "what": html}) is None


def test_non_dict_body_rejected():
    assert validate_submission_payload([]) is not None  # type: ignore[arg-type]
