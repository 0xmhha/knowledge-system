// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Base } from "./Base.sol";

abstract contract Mid is Base {
    uint256 public midValue;

    function _midHook() internal virtual {
        midValue = memberCount;
    }
}
