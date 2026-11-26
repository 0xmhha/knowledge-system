// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IToken {
    event Minted(address indexed to, uint256 amount);

    function mint(address to, uint256 amount) external returns (bool);

    function burn(uint256 amount) external;

    function totalSupply() external view returns (uint256);
}
