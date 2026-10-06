from sanitize import sanitize_what_html, what_for_chat, what_plain_text


def test_keeps_allowlisted_formatting():
    html = "<p>Learned <strong>a lot</strong> from <em>this</em>.</p>"
    assert sanitize_what_html(html) == html


def test_strips_script_tag_and_contents():
    assert sanitize_what_html('<p>hi</p><script>alert("x")</script>') == "<p>hi</p>"


def test_strips_disallowed_attribute_but_keeps_element():
    result = sanitize_what_html('<p onerror="alert(1)">hi</p>')
    assert "onerror" not in result
    assert "hi" in result


def test_keeps_safe_link_href():
    assert 'href="https://wso2.com"' in sanitize_what_html('<a href="https://wso2.com">WSO2</a>')


def test_drops_javascript_link():
    assert "javascript:" not in sanitize_what_html('<a href="javascript:alert(1)">bad</a>')


def test_plain_text_counts_only_text_not_markup():
    assert what_plain_text("<p><strong>abc</strong></p>") == "abc"


def test_plain_text_does_not_glue_paragraphs():
    assert what_plain_text("<p>One</p><p>Two</p>") == "One Two"


def test_plain_text_of_empty_quill_markup_is_empty():
    assert what_plain_text("<p><br></p>") == ""


def test_what_for_chat_converts_inline_tags():
    result = what_for_chat("<p>Learned <strong>a lot</strong> from <em>this</em>.</p>")
    assert "<b>a lot</b>" in result
    assert "<i>this</i>" in result
    assert "<p>" not in result and "<strong>" not in result


def test_what_for_chat_converts_list_items():
    result = what_for_chat("<ul><li>First</li><li>Second</li></ul>")
    assert "• First" in result
    assert "• Second" in result
    assert "<ul>" not in result and "<li>" not in result
