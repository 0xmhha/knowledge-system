// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract GapBase {
    uint256 internal first; // slot 0
    uint256[4] private __gap; // slots 1 to 4
}

contract GapChild is GapBase {
    address internal owner; // slot 5
    mapping(address => bool) internal flags; // slot 6
}

library SetHolder {
    struct Set {
        address[] values; // one slot: the length
        mapping(address => uint256) positions; // one slot
    }
}

contract SetUser {
    SetHolder.Set private blocked; // slots 0 and 1
    SetHolder.Set private allowed; // slots 2 and 3
    address private manager; // slot 4
}

enum Phase {
    Idle,
    Open,
    Closed
}

// A header written over several lines, with a comment inside it.
contract Commented is
    GapBase // the only base
{
    uint256 internal extra; // slot 5
}
