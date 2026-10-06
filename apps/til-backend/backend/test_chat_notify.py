import pytest

import chat_notify


class _FakeResponse:
    status_code = 200


class _FakeAsyncClient:
    calls: list = []

    def __init__(self, *args, **kwargs):
        pass

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        return False

    async def post(self, url, json, timeout):
        _FakeAsyncClient.calls.append({"url": url, "json": json})
        return _FakeResponse()


@pytest.fixture(autouse=True)
def fake_client(monkeypatch):
    _FakeAsyncClient.calls = []
    monkeypatch.setattr(chat_notify.httpx, "AsyncClient", _FakeAsyncClient)
    yield


def _widgets_of(call):
    card = call["json"]["cardsV2"][0]["card"]
    return card["sections"][0]["widgets"]


@pytest.mark.anyio
async def test_posts_nowhere_when_neither_webhook_is_set(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "")
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    assert _FakeAsyncClient.calls == []


@pytest.mark.anyio
async def test_posts_to_collaboration_only_when_only_that_is_set(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "https://chat.googleapis.com/collab")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "")
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    assert [c["url"] for c in _FakeAsyncClient.calls] == ["https://chat.googleapis.com/collab"]


@pytest.mark.anyio
async def test_posts_to_announcement_only_when_only_that_is_set(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "https://chat.googleapis.com/announce")
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    assert [c["url"] for c in _FakeAsyncClient.calls] == ["https://chat.googleapis.com/announce"]


@pytest.mark.anyio
async def test_posts_to_both_spaces_when_both_are_set(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "https://chat.googleapis.com/collab")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "https://chat.googleapis.com/announce")
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    urls = {c["url"] for c in _FakeAsyncClient.calls}
    assert urls == {"https://chat.googleapis.com/collab", "https://chat.googleapis.com/announce"}


@pytest.mark.anyio
async def test_one_dead_webhook_does_not_stop_the_other(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "https://chat.googleapis.com/collab")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "https://chat.googleapis.com/announce")

    import httpx

    class _FlakyAsyncClient(_FakeAsyncClient):
        async def post(self, url, json, timeout):
            if "collab" in url:
                raise httpx.RequestError("dead webhook", request=None)
            return await super().post(url, json, timeout)

    monkeypatch.setattr(chat_notify.httpx, "AsyncClient", _FlakyAsyncClient)
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    assert [c["url"] for c in _FakeAsyncClient.calls] == ["https://chat.googleapis.com/announce"]


@pytest.mark.anyio
async def test_no_button_without_an_entry_url(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "https://chat.googleapis.com/fake")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "")
    await chat_notify.notify_new_submission(who="Jane", where="Internal", what="<p>x</p>")
    widgets = _widgets_of(_FakeAsyncClient.calls[-1])
    assert not any("buttonList" in w for w in widgets)


@pytest.mark.anyio
async def test_button_links_to_the_entry_when_url_given(monkeypatch):
    monkeypatch.setattr(chat_notify, "TIL_CHAT_WEBHOOK_URL", "https://chat.googleapis.com/fake")
    monkeypatch.setattr(chat_notify, "TIL_CHAT_ANNOUNCEMENT_WEBHOOK_URL", "")
    await chat_notify.notify_new_submission(
        who="Jane", where="Internal", what="<p>x</p>", entry_url="http://localhost:3000/knowledge-base/abc123"
    )
    widgets = _widgets_of(_FakeAsyncClient.calls[-1])
    button_widgets = [w for w in widgets if "buttonList" in w]
    assert len(button_widgets) == 1
    button = button_widgets[0]["buttonList"]["buttons"][0]
    assert button["onClick"]["openLink"]["url"] == "http://localhost:3000/knowledge-base/abc123"


@pytest.fixture
def anyio_backend():
    return "asyncio"
