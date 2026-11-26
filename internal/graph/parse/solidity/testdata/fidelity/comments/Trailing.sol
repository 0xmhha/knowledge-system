// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Trailing {
    enum Result {
        Ok, // the call succeeded
        Failed // the call failed
    }

    uint256 public limit; // the most a caller may take

    event Taken(uint256 amount); /* emitted on take */ // and logged

    uint256 public before; /// documents later, not before
    uint256 public later;

    function take() external {} // takes nothing
} // Trailing ends here
