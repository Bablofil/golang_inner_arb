/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"math"
	"strconv"
)

/* Тут в основном идет работа с деревьями */

func insert_leaf(l *leaf, price float64, vol float64) *leaf {
	/* If the tree is empty, return a new node */
	if l == nil {
		return &leaf{left: nil, right: nil, price: price, vol: vol}
	}
	/* Otherwise, recur down the tree */
	if price < l.price {
		l.left = insert_leaf(l.left, price, vol)
	} else {
		if price > l.price {
			l.right = insert_leaf(l.right, price, vol)
		} else {
			l.vol = vol
		}
	}

	/* return the (unchanged) node pointer */

	return l
}

// A utility function to do inorder traversal of BST
func traverse(l *leaf) {
	if l != nil {
		traverse(l.left)
		traverse(l.right)
	}
}

func get_lowest_price(root *leaf) (float64, float64) {
	// Returns maximum value in a given Binary Tree

	// Base case
	if root == nil {
		return math.Inf(0), math.Inf(0)
	}

	// Return maximum of 3 values:
	// 1) Root's data 2) Max in Left Subtree
	// 3) Max in right subtree

	var res float64 = math.Inf(0)
	var vol float64 = 0

	var lres float64
	var lvol float64

	var rres float64
	var rvol float64

	if root.vol > 0 {
		res = root.price
		vol = root.vol
	}

	lres, lvol = get_lowest_price(root.left)
	rres, rvol = get_lowest_price(root.right)
	if lres < res {
		res = lres
		vol = lvol
	}
	if rres < res {
		res = rres
		vol = rvol
	}
	return res, vol

}

func get_highest_price(root *leaf) (float64, float64) {
	// Returns maximum value in a given Binary Tree

	// Base case
	if root == nil {
		return 0, 0
	}

	// Return maximum of 3 values:
	// 1) Root's data 2) Max in Left Subtree
	// 3) Max in right subtree

	var res float64 = 0
	var vol float64 = 0

	if root.vol > 0 {
		res = root.price
		vol = root.vol
	}
	var lres float64
	var lvol float64

	var rres float64
	var rvol float64

	lres, lvol = get_highest_price(root.left)
	rres, rvol = get_highest_price(root.right)
	if lres > res {
		res = lres
		vol = lvol
	}
	if rres > res {
		res = rres
		vol = rvol
	}
	return res, vol

}

func update_tree(dataset [][]string, l *leaf) *leaf {
	var root *leaf = nil
	for _, v := range dataset {
		price, _ := strconv.ParseFloat(v[0], 32)
		vol, _ := strconv.ParseFloat(v[1], 32)
		new_l := insert_leaf(l, price, vol)
		if root == nil {
			root = new_l
		}
	}
	return root
}
