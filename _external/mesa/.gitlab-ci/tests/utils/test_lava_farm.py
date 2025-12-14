from lava.utils.lava_farm import get_lava_farm


def test_get_lava_farm_no_tag(monkeypatch):
    monkeypatch.delenv("FARM", raising=False)
    assert get_lava_farm() == "unknown"
