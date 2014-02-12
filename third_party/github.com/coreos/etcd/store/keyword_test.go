/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"testing"
)

func TestKeywords(t *testing.T) {
	keyword := CheckKeyword("_etcd")
	if !keyword {
		t.Fatal("_etcd should be keyword")
	}

	keyword = CheckKeyword("/_etcd")

	if !keyword {
		t.Fatal("/_etcd should be keyword")
	}

	keyword = CheckKeyword("/_etcd/")

	if !keyword {
		t.Fatal("/_etcd/ contains keyword prefix")
	}

	keyword = CheckKeyword("/_etcd/node1")

	if !keyword {
		t.Fatal("/_etcd/* contains keyword prefix")
	}

	keyword = CheckKeyword("/nokeyword/_etcd/node1")

	if keyword {
		t.Fatal("this does not contain keyword prefix")
	}

}
