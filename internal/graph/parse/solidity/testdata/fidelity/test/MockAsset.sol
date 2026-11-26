// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { IToken } from "../interfaces/IAsset.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract MockToken is IToken, IERC20 {
    uint256 private supply;
    mapping(address => uint256) private balances;

    function mint(address to, uint256 amount) external returns (bool) {
        balances[to] += amount;
        supply += amount;
        emit Minted(to, amount);
        return true;
    }

    function burn(uint256 amount) external {
        supply -= amount;
    }

    function totalSupply() external view returns (uint256) {
        return supply;
    }
}
