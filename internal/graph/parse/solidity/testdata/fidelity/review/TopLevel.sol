// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Thing as T2 } from "./Things.sol";
import "./Things.sol" as Space;

uint256 constant TOP_C = 7;

struct TopS {
    uint256 a;
}

enum TopE {
    A,
    B
}

error TopErr(uint256 a);

event TopEv(uint256 a);

function topFn(TopS memory s) pure returns (uint256) {
    if (s.a > TOP_C) revert TopErr(s.a);
    return s.a + TOP_C;
}

contract C is T2 {
    TopS internal top;
    TopE internal kind;
    Space.Thing internal other;

    function bump() public override limited {
        count += topFn(top);
        if (kind == TopE.B) revert TooMany();
        emit Bumped(count);
        emit TopEv(count);
    }
}
