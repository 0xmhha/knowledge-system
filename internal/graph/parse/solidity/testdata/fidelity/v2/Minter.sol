// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Mid } from "../abstracts/Mid.sol";
import { IToken } from "../interfaces/IAsset.sol";
import { SetLib } from "../libraries/SetLib.sol";

contract Minter is Mid {
    error ZeroAmount();

    event Minted(address indexed to, uint256 amount);

    IToken public token;
    uint256 public minted;
    mapping(address => uint256) public quota;

    modifier whenNotPaused() {
        require(!paused, AlreadyPaused());
        _;
    }

    function mint(address to, uint256 amount) external whenNotPaused returns (bool) {
        if (amount == 0) revert ZeroAmount();
        quota[to] += amount;
        minted = minted + amount;
        emit Minted(to, amount);
        return true;
    }

    function _execute(uint256 id) internal override returns (bool) {
        return id == minted;
    }
}
