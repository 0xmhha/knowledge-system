// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IRegistry {
    function ping(address who) external returns (bool);
}

library Shapes {
    enum Kind {
        Square,
        Circle
    }

    struct Box {
        uint256 width;
        Kind kind;
    }

    uint256 internal constant LIMIT = 4;
}

contract Uses {
    enum Phase {
        Idle,
        Open
    }

    struct Item {
        address owner;
        uint256 size;
        Shapes.Box box;
    }

    Phase public phase;
    IRegistry public registry;
    mapping(uint256 => Item) internal items;
    Item[] internal list;
    Shapes.Box internal top;

    function open(uint256 id, address who) external returns (Phase) {
        phase = Phase.Open;
        Item storage item = items[id];
        item.owner = who;
        items[id].size = Shapes.LIMIT;
        item.box.width = list[0].size + top.width;
        if (item.box.kind == Shapes.Kind.Circle) {
            registry = IRegistry(who);
        }
        Item memory copy = Item(who, 1, Shapes.Box(2, Shapes.Kind.Square));
        list.push(copy);
        return phase;
    }
}
