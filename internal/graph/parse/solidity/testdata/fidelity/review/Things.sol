// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Thing {
    uint256 internal count;

    event Bumped(uint256 count);

    error TooMany();

    modifier limited() {
        if (count > 3) revert TooMany();
        _;
    }

    function bump() public virtual limited {
        count += 1;
        emit Bumped(count);
    }
}
