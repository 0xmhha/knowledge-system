// SPDX-License-Identifier: MIT
pragma solidity ^0.6.12;

// Before 0.7 a struct with a mapping member can be constructed: the
// constructor takes the other members and leaves the mapping out.
contract Legacy {
    struct Shelf {
        uint256 count;
        mapping(uint256 => uint256) slots;
        address keeper;
    }

    Shelf internal shelf;

    function reset(address keeper) external {
        shelf = Shelf(0, keeper);
    }
}
