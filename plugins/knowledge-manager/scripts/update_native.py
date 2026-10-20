"""Typed native tool results shared by update workers and retained readers."""
from manager import Failure,require


class Refusal(Failure):
    def __init__(self,value):
        self.value=value
        super().__init__(value['code'])


def value(response):
    payload=response.get('structuredContent')
    require(isinstance(payload,dict),'native_protocol_invalid')
    if response.get('isError'):
        require(isinstance(payload.get('code'),str),'native_protocol_invalid')
        raise Refusal(payload)
    return payload


def code_inventory():
    """Long-lived processes stop using changed on-disk manager code."""
    from pathlib import Path
    from manager import file_hash
    files=sorted(Path(__file__).resolve().parent.glob('*.py'))
    require(files and all(p.resolve()==p and not p.is_symlink() and p.stat().st_nlink==1 for p in files),'manager_code_changed')
    return {str(p):file_hash(p)for p in files}


def check_code(expected):
    require(code_inventory()==expected,'manager_code_changed')
