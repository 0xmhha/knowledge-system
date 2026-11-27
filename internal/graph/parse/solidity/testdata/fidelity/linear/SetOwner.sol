// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { SetHolder, Phase } from "./Gap.sol";

contract SetOwner {
    SetHolder.Set private first; // slots 0 and 1: the struct is declared in another file
    address private second; // slot 2, 20 bytes
    Phase private phase; // slot 2 too: an enum of three values is one byte
    bool private done; // slot 2 too
    uint256 private third; // slot 3
}
